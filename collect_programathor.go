package main

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// programathorCollector scrapes the Programathor HTML listing pages.
// The old JSON feeds (/jobs-json, /jobs-api) now return server-rendered HTML,
// so we parse the DOM directly — no headless browser needed.
type programathorCollector struct{}

func (p *programathorCollector) name() string { return "programathor" }
func (p *programathorCollector) kind() string { return "scraping" }

func (p *programathorCollector) collect() ([]Job, error) {
	var all []Job
	page := 1
	for {
		jobs, err := p.fetchPage(page)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		if len(jobs) == 0 {
			break
		}
		all = append(all, jobs...)
		page++
		if page > 20 {
			break
		}
	}
	return all, nil
}

func (p *programathorCollector) fetchPage(page int) ([]Job, error) {
	u := fmt.Sprintf("https://programathor.com.br/jobs-json?page=%d", page)
	resp, err := clientGet(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var jobs []Job
	doc.Find("a[href^='/jobs/']").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || href == "" {
			return
		}

		j := p.parseCard(s, href)
		if j.Title != "" {
			jobs = append(jobs, j)
		}
	})
	return jobs, nil
}

func (p *programathorCollector) parseCard(s *goquery.Selection, href string) Job {
	// Extract ID from /jobs/<id>-<slug>
	id := extractID(href)

	// Title is inside h3. Badges (PRESENCIAL, Vencida) are <span> children,
	// so remove them before reading the text — this keeps titles like
	// "Desenvolvedor(a) .NET Júnior" intact instead of cutting at the level.
	h3 := s.Find("h3").Clone()
	h3.Find("span").Remove()
	title := strings.TrimSpace(h3.Text())
	if title == "" {
		return Job{}
	}

	// Strip any leftover badge text from title
	title = cleanTitle(title)

	// Company, location, salary, type — from spans with icons
	var company, location, salary, companyType string
	var isRemote bool

	s.Find("span").Each(func(_ int, sp *goquery.Selection) {
		text := strings.TrimSpace(sp.Text())
		if strings.Contains(text, "fa-briefcase") || sp.HasClass("fa-briefcase") {
			company = strings.TrimSpace(strings.ReplaceAll(text, "fa-briefcase", ""))
		}
		if strings.Contains(text, "fa-map-marker") || strings.Contains(text, "fa-building") || strings.Contains(text, "fa-money") {
			// These are icon + text spans
		}
	})

	// Extract from the cell-list-content-icon section
	icons := s.Find(".cell-list-content-icon span")
	icons.Each(func(_ int, sp *goquery.Selection) {
		text := strings.TrimSpace(sp.Text())
		switch {
		case strings.Contains(text, "briefcase"):
			company = strings.TrimSpace(strings.TrimPrefix(text, "\uf0b1"))
		case strings.Contains(text, "map-marker") || strings.Contains(text, "map-pin"):
			location = strings.TrimSpace(strings.TrimPrefix(text, "\uf3c5"))
			if strings.Contains(strings.ToLower(location), "presencial") || strings.Contains(strings.ToLower(text), "presencial") {
				isRemote = false
			}
		case strings.Contains(text, "building"):
			companyType = strings.TrimSpace(strings.TrimPrefix(text, "\uf1ad"))
		case strings.Contains(text, "money"):
			salary = strings.TrimSpace(strings.TrimPrefix(text, "\uf571"))
		}
	})

	// Remote from badges
	if strings.Contains(strings.ToLower(s.Text()), "remoto") {
		isRemote = true
	}
	if strings.Contains(strings.ToLower(title), "remoto") || strings.Contains(strings.ToLower(title), "remote") {
		isRemote = true
	}
	// Check for presencial
	if strings.Contains(strings.ToLower(title), "presencial") || strings.Contains(strings.ToLower(s.Text()), "presencial") {
		isRemote = false
	}

	// Extract tags/skills
	var tags []string
	s.Find(".cell-list-content-icon span").Each(func(_ int, sp *goquery.Selection) {
		text := strings.TrimSpace(sp.Text())
		// Tags are the short skill names (Python, React, etc.)
		if len(text) > 0 && len(text) < 30 && !strings.Contains(text, "fa-") &&
			!strings.Contains(text, "fa ") &&
			text != company && text != location && text != salary && text != companyType {
			// Filter out non-tag items
			if !strings.Contains(text, "Remoto") && !strings.Contains(text, "Presencial") &&
				!strings.Contains(text, "CLT") && !strings.Contains(text, "PJ") &&
				!strings.Contains(text, "Júnior") && !strings.Contains(text, "Pleno") &&
				!strings.Contains(text, "Sênior") && !strings.Contains(text, "Estágio") &&
				!strings.Contains(text, "Até") && !strings.Contains(text, "Startup") &&
				!strings.Contains(text, "Grande empresa") && !strings.Contains(text, "Pequena") &&
				!strings.Contains(text, "Aceito") && !strings.Contains(text, "Não") &&
				!strings.Contains(text, "Aceita") {
				tags = append(tags, text)
			}
		}
	})

	// Clean up location
	if location == "" {
		if isRemote {
			location = "remoto"
		}
	}

	raw := strings.Join(tags, " ")
	if salary != "" {
		raw += " " + salary
	}
	if companyType != "" {
		raw += " " + companyType
	}

	return Job{
		Source:   "programathor",
		ID:       id,
		URL:      "https://programathor.com.br" + href,
		Title:    title,
		Company:  company,
		Location: location,
		Remote:   isRemote,
		Tags:     tags,
		Raw:      raw,
	}
}

// extractID pulls the numeric ID from /jobs/<id>-<slug>
func extractID(href string) string {
	_, after, _ := strings.Cut(href, "/jobs/")
	if after == "" {
		return ""
	}
	idStr, _, _ := strings.Cut(after, "-")
	return idStr
}

// cleanTitle removes leftover badge/fragment text from a job title.
func cleanTitle(title string) string {
	// Common badge prefixes that may survive as raw text on some pages
	for _, prefix := range []string{
		"📍 PRESENCIAL - SOMENTE PARA CANDIDATOS NO LOCAL",
		"PRESENCIAL - SOMENTE PARA CANDIDATOS NO LOCAL",
		"📍 PRESENCIAL",
		"PRESENCIAL",
	} {
		title = strings.ReplaceAll(title, prefix, "")
	}
	// Remove Vencida badge
	title = strings.ReplaceAll(title, "Vencida", "")
	return strings.TrimSpace(title)
}
