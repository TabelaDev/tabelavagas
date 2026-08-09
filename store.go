package main

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store persists normalized jobs in a local SQLite database.
type Store struct {
	db *sql.DB
}

func defaultDBPath() string {
	if p := os.Getenv("TABELAVAGAS_DB"); p != "" {
		return p
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state", "tabelavagas", "vagas.db")
}

func openStore() (*Store, error) {
	return openStoreAt(defaultDBPath())
}

func openStoreAt(path string) (*Store, error) {
	os.MkdirAll(filepath.Dir(path), 0o755)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
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
	return nil
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

// save inserts jobs that are not already present; returns inserts count.
func (s *Store) save(jobs []Job) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO jobs
		(source, id, url, title, company, location, remote, type, deadline, salary, description, raw)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, j := range jobs {
		res, err := stmt.Exec(j.Source, j.ID, j.URL, j.Title, j.Company, j.Location, b2i(j.Remote), j.Type, j.Deadline, j.Salary, truncate(j.Description, 4000), truncate(j.Raw, 2000))
		if err != nil {
			return inserted, err
		}
		n, _ := res.RowsAffected()
		inserted += int(n)
	}
	return inserted, tx.Commit()
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

// scoreAllLLMProgress is scoreAllLLM with a per-job progress callback.
func scoreAllLLMProgress(s *Store, sc Scorer, onJob func(done, total int)) ([]Job, error) {
	jobs, err := s.all()
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		jobs[i].Score = sc.Score(jobs[i])
		if onJob != nil {
			onJob(i+1, len(jobs))
		}
	}
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
	if err := rows.Scan(&j.Source, &j.ID, &j.URL, &j.Title, &j.Company, &j.Location, &remote, &j.Type, &j.Deadline, &j.Salary, &j.Description, &j.Raw, &score, &vetoed); err != nil {
		return j, err
	}
	j.Remote = remote == 1
	j.Vetoed = vetoed == 1
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
