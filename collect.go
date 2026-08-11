package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Output sinks. The CLI writes to the real streams; the TUI swaps them out
// while a command runs, because anything printed under a Bubble Tea alt-screen
// lands on top of the rendered frame and garbles it. collect, notify and the
// LLM scorer run from both entry points, and the LLM scorer prints one warning
// per job — an invalid API key used to turn the whole screen to soup.
var (
	outWriter io.Writer = os.Stdout
	errWriter io.Writer = os.Stderr
	writerMu  sync.Mutex
)

func stderr() io.Writer { return errWriter }
func stdout() io.Writer { return outWriter }

// lockedWriter serializes writes, since the LLM scorer fans out across workers.
type lockedWriter struct {
	mu  *sync.Mutex
	buf *strings.Builder
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// captureOutput redirects both sinks into a buffer and returns a function that
// restores them and yields whatever was written. The TUI uses it to turn stray
// prints into activity-log lines instead of screen corruption.
func captureOutput() func() string {
	writerMu.Lock()
	var buf strings.Builder
	var mu sync.Mutex
	prevOut, prevErr := outWriter, errWriter
	outWriter = &lockedWriter{mu: &mu, buf: &buf}
	errWriter = outWriter
	writerMu.Unlock()

	return func() string {
		writerMu.Lock()
		outWriter, errWriter = prevOut, prevErr
		writerMu.Unlock()

		mu.Lock()
		defer mu.Unlock()
		return strings.TrimSpace(buf.String())
	}
}

// userAgent identifies the collector to the sites it scrapes. Go's default
// ("Go-http-client/1.1") is blocked by a good share of job boards.
const userAgent = "tabelavagas/0.3 (+https://github.com/TabelaDev/tabelavagas)"

// httpClient is built at init with the default timeout, because package vars
// are evaluated before main can read config.toml; loadSettings swaps in the
// configured one through apply().
var httpClient = newHTTPClient(defaultConfig().Collector.HTTPTimeout.Duration)

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func clientGet(rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
	return httpClient.Do(req)
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
