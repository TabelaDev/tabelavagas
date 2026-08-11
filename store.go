package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ianptkcs/tabelatuiui"
	_ "modernc.org/sqlite"
)

// Store persists normalized jobs in a local SQLite database.
type Store struct {
	db *sql.DB
}

// defaultDBPath comes from config.toml, with TABELAVAGAS_DB still winning
// over the file.
func defaultDBPath() string {
	return tuiui.ExpandHome(envOr("TABELAVAGAS_DB", settings.Database.Path))
}

func openStore() (*Store, error) {
	return openStoreAt(defaultDBPath())
}

func openStoreAt(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("criar %s: %w", filepath.Dir(path), err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// One connection: SQLite serializes writers anyway, and the LLM scorer now
	// scores jobs from a worker pool, each caching its result. Letting the pool
	// open several connections just turns that into SQLITE_BUSY.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS jobs (
	source  TEXT NOT NULL,
	id      TEXT NOT NULL,
	url     TEXT,
	title   TEXT,
	company TEXT,
	location TEXT,
	remote  INTEGER,
	type    TEXT,
	deadline TEXT,
	raw     TEXT,
	seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	score   REAL,
	notified INTEGER DEFAULT 0,
	vetoed  INTEGER DEFAULT 0,
	llm_score REAL,
	llm_hash TEXT,
	PRIMARY KEY (source, id)
);
CREATE INDEX IF NOT EXISTS idx_jobs_score ON jobs(score);`); err != nil {
		return err
	}
	// Pre-existing DBs created by older builds may miss columns that newer
	// code expects (vetoed, llm_score, llm_hash) — add them in place.
	for _, cc := range []struct{ col, decl string }{
		{"vetoed", "INTEGER DEFAULT 0"},
		{"llm_score", "REAL"},
		{"llm_hash", "TEXT"},
		{"salary", "TEXT"},
		{"description", "TEXT"},
	} {
		if err := s.ensureColumn("jobs", cc.col, cc.decl); err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS activity (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	at DATETIME DEFAULT CURRENT_TIMESTAMP,
	kind TEXT NOT NULL,
	detail TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activity_at ON activity(at);`)
	return err
}

// ensureColumn adds col with decl to table if it doesn't already exist.
func (s *Store) ensureColumn(table, col, decl string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + decl)
	return err
}

// save upserts jobs, returning how many rows were new.
//
// It used to be INSERT OR IGNORE, which meant a posting that changed after the
// first sync — salary published later, deadline moved, title corrected — was
// never updated. The conflict branch refreshes the volatile fields and
// deliberately leaves notified, vetoed and both score columns alone: those are
// the user's state, not the source's.
func (s *Store) save(jobs []Job) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO jobs
		(source, id, url, title, company, location, remote, type, deadline, salary, description, raw)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(source, id) DO UPDATE SET
			url = excluded.url,
			title = excluded.title,
			company = excluded.company,
			location = excluded.location,
			remote = excluded.remote,
			type = excluded.type,
			deadline = excluded.deadline,
			salary = excluded.salary,
			description = excluded.description,
			raw = excluded.raw
		WHERE
			jobs.url IS NOT excluded.url OR
			jobs.title IS NOT excluded.title OR
			jobs.company IS NOT excluded.company OR
			jobs.location IS NOT excluded.location OR
			jobs.remote IS NOT excluded.remote OR
			jobs.type IS NOT excluded.type OR
			jobs.deadline IS NOT excluded.deadline OR
			jobs.salary IS NOT excluded.salary OR
			jobs.description IS NOT excluded.description OR
			jobs.raw IS NOT excluded.raw`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	// RowsAffected counts updates too, so "new" is measured by how many rows
	// the table gained rather than by how many statements touched something.
	var before int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&before); err != nil {
		return 0, err
	}

	for _, j := range jobs {
		if _, err := stmt.Exec(j.Source, j.ID, j.URL, j.Title, j.Company, j.Location, b2i(j.Remote), j.Type, j.Deadline, j.Salary, truncate(j.Description, 4000), truncate(j.Raw, 2000)); err != nil {
			return 0, err
		}
	}

	var after int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&after); err != nil {
		return 0, err
	}

	return after - before, tx.Commit()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func (s *Store) close() { s.db.Close() }

// top returns up to n jobs with the highest score (already scored).
func (s *Store) top(n int) ([]Job, error) {
	return s.topFiltered(n, 0, false, false)
}

// topUnnotified returns up to n jobs with the highest score that have not
// been notified yet.
func (s *Store) topUnnotified(n int) ([]Job, error) {
	return s.topFiltered(n, 0, true, false)
}

// topFiltered returns up to n scored jobs, optionally filtered by a minimum
// score, excluding jobs already notified, and/or including vetoed jobs.
func (s *Store) topFiltered(n, minScore int, onlyUnnotified, includeVetoed bool) ([]Job, error) {
	q := `SELECT source, id, url, title, company, location, remote, type, deadline, salary, description, raw, score, vetoed
		FROM jobs WHERE score IS NOT NULL`
	args := []any{}
	if minScore > 0 {
		q += ` AND score >= ?`
		args = append(args, minScore)
	}
	if onlyUnnotified {
		q += ` AND notified = 0`
	}
	if !includeVetoed {
		q += ` AND vetoed = 0`
	}
	q += ` ORDER BY score DESC LIMIT ?`
	args = append(args, n)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// setVetoed marks (or unmarks) a job as vetoed.
func (s *Store) setVetoed(source, id string, v bool) error {
	_, err := s.db.Exec(`UPDATE jobs SET vetoed = ? WHERE source = ? AND id = ?`, b2i(v), source, id)
	return err
}

// scoreAll recomputes the score for every stored job using the given scorer
// and writes them to the active score column.
func scoreAll(s *Store, sc Scorer) error {
	jobs, err := s.all()
	if err != nil {
		return err
	}
	for i := range jobs {
		jobs[i].Score = sc.Score(jobs[i])
	}
	return s.applyScores(jobs)
}

// applyScores writes the given jobs' scores into the active score column.
func (s *Store) applyScores(jobs []Job) error {
	if len(jobs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE jobs SET score = ? WHERE source = ? AND id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, j := range jobs {
		if _, err := stmt.Exec(j.Score, j.Source, j.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// scoreAllLLM computes (and caches in llm_score/llm_hash) the LLM score for
// every job WITHOUT touching the active score column — the caller decides
// whether to apply. Re-runs are free: the LLM scorer reads its cache.
func scoreAllLLM(s *Store, sc Scorer) ([]Job, error) {
	return scoreAllLLMProgress(s, sc, nil)
}

// llmScoreWorkers bounds the fan-out of LLM scoring ([llm].workers). Each
// Score is an HTTP round trip, so scoring a few hundred stored jobs strictly
// one at a time was effectively unusable; a small pool keeps it quick without
// hammering the provider. It's read when a run starts, so a reload only
// affects the next one.
func llmScoreWorkers() int { return settings.LLM.Workers }

// scoreAllLLMProgress is scoreAllLLM with a per-job progress callback. Jobs are
// scored concurrently, but each result is written back to its own slice slot,
// so the caller still gets them in the original order.
func scoreAllLLMProgress(s *Store, sc Scorer, onJob func(done, total int)) ([]Job, error) {
	jobs, err := s.all()
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return jobs, nil
	}

	workers := min(llmScoreWorkers(), len(jobs))

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		done    int
		indexes = make(chan int)
	)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				score := sc.Score(jobs[i])

				mu.Lock()
				jobs[i].Score = score
				done++
				current := done
				mu.Unlock()

				if onJob != nil {
					onJob(current, len(jobs))
				}
			}
		}()
	}

	for i := range jobs {
		indexes <- i
	}
	close(indexes)
	wg.Wait()

	return jobs, nil
}

func (s *Store) all() ([]Job, error) {
	rows, err := s.db.Query(`SELECT source, id, url, title, company, location, remote, type, deadline, salary, description, raw, score, vetoed FROM jobs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func scanJob(rows *sql.Rows) (Job, error) {
	var j Job
	var remote, vetoed int
	var score sql.NullInt64
	var salary, description, raw sql.NullString
	if err := rows.Scan(&j.Source, &j.ID, &j.URL, &j.Title, &j.Company, &j.Location, &remote, &j.Type, &j.Deadline, &salary, &description, &raw, &score, &vetoed); err != nil {
		return j, err
	}
	j.Remote = remote == 1
	j.Vetoed = vetoed == 1
	j.Salary = salary.String
	j.Description = description.String
	j.Raw = raw.String
	if score.Valid {
		j.Score = int(score.Int64)
	} else {
		j.Score = 0
	}
	return j, nil
}

// setDetails caches per-job detail fields (salary, description) fetched
// lazily from the source page.
func (s *Store) setDetails(source, id, salary, description string) error {
	_, err := s.db.Exec(`UPDATE jobs SET salary = ?, description = ? WHERE source = ? AND id = ?`, salary, description, source, id)
	return err
}

// llmCachedScore returns the cached LLM score and content hash for a job.
// Returns (score, hash, true) if cached, (0, "", false) otherwise.
func (s *Store) llmCachedScore(source, id string) (float64, string, bool) {
	var score sql.NullFloat64
	var hash sql.NullString
	err := s.db.QueryRow(`SELECT llm_score, llm_hash FROM jobs WHERE source = ? AND id = ?`, source, id).Scan(&score, &hash)
	if err != nil {
		return 0, "", false
	}
	if !score.Valid || !hash.Valid {
		return 0, "", false
	}
	return score.Float64, hash.String, true
}

// setLLMScore caches the LLM score and content hash for a job.
func (s *Store) setLLMScore(source, id string, score float64, hash string) error {
	_, err := s.db.Exec(`UPDATE jobs SET llm_score = ?, llm_hash = ? WHERE source = ? AND id = ?`, score, hash, source, id)
	return err
}

// markNotified marks the given jobs as already notified so they are not
// picked up again by --only-new.
func (s *Store) markNotified(jobs []Job) error {
	if len(jobs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE jobs SET notified = 1 WHERE source = ? AND id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, j := range jobs {
		if _, err := stmt.Exec(j.Source, j.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// activity is one entry in the persistent activity log.
type activity struct {
	At     time.Time
	Kind   string
	Detail string
}

// addActivity records an activity and prunes entries older than 7 days.
func (s *Store) addActivity(kind, detail string) error {
	if _, err := s.db.Exec(`INSERT INTO activity (kind, detail) VALUES (?, ?)`, kind, detail); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM activity WHERE at < datetime('now', '-7 days')`)
	return err
}

// recentActivity returns the last n activities, newest first.
func (s *Store) recentActivity(n int) ([]activity, error) {
	rows, err := s.db.Query(`SELECT at, kind, detail FROM activity ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []activity
	for rows.Next() {
		var a activity
		if err := rows.Scan(&a.At, &a.Kind, &a.Detail); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// persistActivity records an activity from anywhere (CLI/TUI), opening the
// store lazily.
func persistActivity(kind, detail string) {
	store, err := openStore()
	if err != nil {
		return
	}
	defer store.close()
	_ = store.addActivity(kind, detail)
}
