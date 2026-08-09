package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func stderr() io.Writer { return os.Stderr }
func stdout() io.Writer { return os.Stdout }

var httpClient = &http.Client{Timeout: 30 * time.Second}

func clientGet(rawURL string) (*http.Response, error) {
	return httpClient.Get(rawURL)
}

// runCollect runs all collectors, dedups by key, saves and returns count.
func runCollect() (int, error) {
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
			fmt.Fprintf(stderr(), "aviso %s: %v\n", c.name(), err)
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
		fmt.Fprintf(stdout(), "%s: %d novas (dedup)\n", c.name(), added)
	}
	n, err := store.save(all)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func allCollectors() []collector {
	return []collector{
		&remotiveCollector{},
		&gupyCollector{},
		&programathorCollector{},
	}
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

// gupyAPIEndpoint builds the Gupy official API URL (api.gupy.io/api/v1/jobs).
// Official docs: https://developers.gupy.io/reference/findjobs
func gupyAPIEndpoint(page int) string {
	u := url.Values{}
	u.Set("fields", "all")
	u.Set("perPage", "50")
	u.Set("page", fmt.Sprintf("%d", page))
	u.Set("status", "published")
	return fmt.Sprintf("https://api.gupy.io/api/v1/jobs?%s", u.Encode())
}

// gupyCollector uses Gupy's official public API (Bearer token via env
// TABELAVAGAS_GUPY_TOKEN). When no token is set it cannot list jobs today
// (the old per-subdomain endpoint was retired) so it reports a clear error.
type gupyCollector struct{}

func (g *gupyCollector) name() string { return "gupy" }
func (g *gupyCollector) kind() string { return "api" } // api.gupy.io (docs oficiais, Bearer)

func (g *gupyCollector) collect() ([]Job, error) {
	token := os.Getenv("TABELAVAGAS_GUPY_TOKEN")
	if token == "" {
		return nil, errors.New("TABELAVAGAS_GUPY_TOKEN não definido; obtenha um token em developers.gupy.io")
	}
	var jobs []Job
	page := 1
	for {
		j, more, err := g.fetchPage(page, token)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j...)
		if !more {
			break
		}
		page++
	}
	return jobs, nil
}

func (g *gupyCollector) fetchPage(page int, token string) ([]Job, bool, error) {
	req, err := http.NewRequest("GET", gupyAPIEndpoint(page), nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, false, fmt.Errorf("http %d: token Gupy inválida", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	var payload struct {
		Results      []gupyJob `json:"results"`
		TotalPages   int       `json:"totalPages"`
		TotalResults int       `json:"totalResults"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, false, err
	}
	var jobs []Job
	for _, d := range payload.Results {
		j := g.jobFrom(d)
		if j.Title != "" {
			jobs = append(jobs, j)
		}
	}
	more := page < payload.TotalPages
	return jobs, more, nil
}

type gupyJob struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	DepartmentName      string `json:"departmentName"`
	RoleName            string `json:"roleName"`
	WorkplaceType       string `json:"workplaceType"`
	CareerPageName      string `json:"careerPageName"`
	AddressCity         string `json:"addressCity"`
	AddressState        string `json:"addressState"`
	AddressCountryShort string `json:"addressCountryShortName"`
	ApplicationDeadline string `json:"applicationDeadline"`
	Prerequisites       string `json:"prerequisites"`
}

func (g *gupyCollector) jobFrom(d gupyJob) Job {
	loc := strings.TrimSpace(d.AddressCity)
	if d.AddressState != "" {
		if loc != "" {
			loc += ", "
		}
		loc += d.AddressState
	}
	if loc == "" {
		loc = d.AddressCountryShort
	}
	remote := strings.EqualFold(d.WorkplaceType, "remote")
	raw := strings.TrimSpace(d.Prerequisites + " " + d.DepartmentName + " " + d.RoleName)
	return Job{
		Source:   "gupy",
		ID:       fmt.Sprintf("%d", d.ID),
		URL:      fmt.Sprintf("https://app.gupy.io/job-posting/%d", d.ID),
		Title:    strings.TrimSpace(d.Name),
		Company:  d.CareerPageName,
		Location: loc,
		Remote:   remote,
		Deadline: d.ApplicationDeadline,
		Raw:      raw,
	}
}
