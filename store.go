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
	_, err := s.db.Exec(`
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
	llm_score REAL,
	llm_hash TEXT,
	PRIMARY KEY (source, id)
);
CREATE INDEX IF NOT EXISTS idx_jobs_score ON jobs(score);`)
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
		(source, id, url, title, company, location, remote, type, deadline, raw)
		VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, j := range jobs {
		res, err := stmt.Exec(j.Source, j.ID, j.URL, j.Title, j.Company, j.Location, b2i(j.Remote), j.Type, j.Deadline, truncate(j.Raw, 2000))
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
	return s.topWhere(n, "")
}

// topUnnotified returns up to n jobs with the highest score that have not
// been notified yet.
func (s *Store) topUnnotified(n int) ([]Job, error) {
	return s.topWhere(n, "AND notified = 0")
}

func (s *Store) topWhere(n int, extra string) ([]Job, error) {
	rows, err := s.db.Query(`SELECT source, id, url, title, company, location, remote, type, deadline, raw, score
		FROM jobs WHERE score IS NOT NULL `+extra+` ORDER BY score DESC LIMIT ?`, n)
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

// scoreAll recomputes the score for every stored job using the given scorer.
func scoreAll(s *Store, sc Scorer) error {
	jobs, err := s.all()
	if err != nil {
		return err
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
		j.Score = sc.Score(j)
		if _, err := stmt.Exec(j.Score, j.Source, j.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) all() ([]Job, error) {
	rows, err := s.db.Query(`SELECT source, id, url, title, company, location, remote, type, deadline, raw, score FROM jobs`)
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
	var remote int
	var score sql.NullInt64
	if err := rows.Scan(&j.Source, &j.ID, &j.URL, &j.Title, &j.Company, &j.Location, &remote, &j.Type, &j.Deadline, &j.Raw, &score); err != nil {
		return j, err
	}
	j.Remote = remote == 1
	if score.Valid {
		j.Score = int(score.Int64)
	} else {
		j.Score = 0
	}
	return j, nil
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
