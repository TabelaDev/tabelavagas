package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// greenhouseCollector fetches job boards from the public Greenhouse API
// (boards-api.greenhouse.io, no auth) for each configured company.
// https://developers.greenhouse.io/job-board.html
type greenhouseCollector struct {
	companies []string
}

func (g *greenhouseCollector) name() string { return "greenhouse" }
func (g *greenhouseCollector) kind() string { return "api" } // boards-api.greenhouse.io (JSON público, sem auth)

func (g *greenhouseCollector) collect() ([]Job, error) {
	var all []Job
	for _, co := range g.companies {
		jobs, err := g.fetchBoard(co)
		if err != nil {
			fmt.Fprintf(stderr(), "aviso greenhouse/%s: %v\n", co, err)
			continue
		}
		all = append(all, jobs...)
		time.Sleep(500 * time.Millisecond) // polite; one request per board
	}
	return all, nil
}

type greenhouseResponse struct {
	Jobs []greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company_name"`
	URL      string `json:"absolute_url"`
	Content  string `json:"content"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Metadata []struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Deadline string `json:"application_deadline"`
}

func (g *greenhouseCollector) fetchBoard(company string) ([]Job, error) {
	url := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs?content=true", company)
	resp, err := clientGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var payload greenhouseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	var jobs []Job
	for _, j := range payload.Jobs {
		title := strings.TrimSpace(j.Title)
		if title == "" {
			continue
		}
		var tags []string
		for _, m := range j.Metadata {
			if n := strings.TrimSpace(m.Name); n != "" {
				tags = append(tags, n)
			}
		}
		loc := strings.TrimSpace(j.Location.Name)
		remote := strings.Contains(strings.ToLower(loc), "remote")
		jobs = append(jobs, Job{
			Source:      "greenhouse",
			ID:          fmt.Sprintf("%s-%d", company, j.ID),
			URL:         j.URL,
			Title:       title,
			Company:     j.Company,
			Location:    loc,
			Remote:      remote,
			Deadline:    j.Deadline,
			Tags:        tags,
			Description: htmlToText(j.Content),
		})
	}
	return jobs, nil
}

// htmlToText strips HTML tags from the job description into plain text.
func htmlToText(h string) string {
	if strings.TrimSpace(h) == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(h))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Text())
}
