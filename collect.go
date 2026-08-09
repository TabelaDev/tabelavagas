package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func stderr() io.Writer { return os.Stderr }
func stdout() io.Writer { return os.Stdout }

var httpClient = &http.Client{Timeout: 30 * time.Second}

func clientGet(rawURL string) (*http.Response, error) {
	return httpClient.Get(rawURL)
}

// collectProgressFunc is called once per source with either the number of new
// jobs (err == nil) or the source failure.
type collectProgressFunc func(source string, added int, err error)

// runCollect runs all collectors, dedups by key, saves and returns count.
func runCollect() (int, error) {
	return runCollectProgress(nil)
}

// runCollectProgress is runCollect with a progress callback, so callers can
// surface per-source results live (the TUI streams them).
func runCollectProgress(prog collectProgressFunc) (int, error) {
	store, err := openStore()
	if err != nil {
		return 0, err
	}
	defer store.close()

	cols := allCollectors()
	seen := map[string]bool{}
	var all []Job
	total := 0
	for _, c := range cols {
		jobs, err := c.collect()
		if err != nil {
			if prog != nil {
				prog(c.name(), 0, err)
			} else {
				fmt.Fprintf(stderr(), "aviso %s: %v\n", c.name(), err)
			}
			continue
		}
		added := 0
		for _, j := range jobs {
			if k := j.key(); !seen[k] {
				seen[k] = true
				all = append(all, j)
				added++
			}
		}
		total += added
		if prog != nil {
			prog(c.name(), added, nil)
		} else {
			fmt.Fprintf(stdout(), "%s: %d novas (dedup)\n", c.name(), added)
		}
	}
	n, err := store.save(all)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func allCollectors() []collector {
	cols := []collector{
		&remotiveCollector{},
		&programathorCollector{},
	}
	// Per-company API boards are opt-in via ~/.config/tabelavagas/sources.toml.
	if companies := loadCompanies("greenhouse"); len(companies) > 0 {
		cols = append(cols, &greenhouseCollector{companies: companies})
	}
	return cols
}

// printSources lists each configured source with its kind (api vs scraping).
func printSources() {
	fmt.Fprintln(stdout(), "fontes configuradas:")
	for _, c := range allCollectors() {
		fmt.Fprintf(stdout(), "  %-14s %s\n", c.name(), c.kind())
	}
}

// collector fetches vacancies from a source and returns normalized jobs.
type collector interface {
	name() string
	kind() string // "api" when an official/public structured endpoint, "scraping" otherwise
	collect() ([]Job, error)
}
