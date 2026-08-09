package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// remotiveCollector uses the Remotive public API (documented JSON, no auth)
// for remote jobs — https://remotive.com/api . It's reliable enough to demo
// the pipeline end-to-end; Gupy (Bearer) and the BR SPAs are extras.
type remotiveCollector struct{}

func (r *remotiveCollector) name() string { return "remotive" }
func (r *remotiveCollector) kind() string { return "api" } // remotive.com/api (JSON público, sem auth)

func (r *remotiveCollector) collect() ([]Job, error) {
	resp, err := get("https://remotive.com/api/remote-jobs?limit=100")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Jobs []struct {
			ID      int      `json:"id"`
			Title   string   `json:"title"`
			Company string   `json:"company_name"`
			URL     string   `json:"url"`
			Tags    []string `json:"tags"`
			Type    string   `json:"job_type"`
			Loc     string   `json:"candidate_required_location"` // e.g. "USA", "Brazil"
		} `json:"jobs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var jobs []Job
	for _, j := range payload.Jobs {
		title := strings.TrimSpace(j.Title)
		if title == "" {
			continue
		}
		loc := j.Loc
		if loc == "" {
			loc = "remoto"
		}
		jobs = append(jobs, Job{
			Source:   "remotive",
			ID:       fmt.Sprintf("%d", j.ID),
			URL:      j.URL,
			Title:    title,
			Company:  j.Company,
			Location: loc,
			Remote:   true,
			Type:     j.Type,
			Tags:     j.Tags,
			Raw:      strings.Join(j.Tags, " "),
		})
	}
	return jobs, nil
}
