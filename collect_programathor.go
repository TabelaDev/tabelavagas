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

	// Company, location, salary, type and tags come from the card's icon
	// spans. Single pass: each span is classified into exactly one bucket.
	var company, location, salary, companyType string
	var isRemote bool
	var tags []string

	s.Find(".cell-list-content-icon span").Each(func(_ int, sp *goquery.Selection) {
		// The Font Awesome icon lives in the <i> child (or the span itself);
		// match on its class, not on the span text which is the label
		// (e.g. "Almeida Kruger", "Paraná (Presencial)", "Até R$2.500").
		icon := sp.AttrOr("class", "") + " " + sp.Find("i").AttrOr("class", "")
		text := strings.TrimSpace(sp.Text())
		switch {
		case strings.Contains(icon, "briefcase"):
			company = text
		case strings.Contains(icon, "map-marker") || strings.Contains(icon, "map-pin"):
			location = strings.TrimSpace(strings.ReplaceAll(text, "(Presencial)", ""))
			if strings.Contains(strings.ToLower(text), "presencial") {
				isRemote = false
			}
		case strings.Contains(icon, "building"):
			companyType = text
		case strings.Contains(icon, "money"):
			salary = text
		default:
			if isTagText(text) {
				tags = append(tags, text)
			}
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

	// Clean up location
	if location == "" {
		if isRemote {
			location = "remoto"
		}
	}

	// Raw carries only the non-tag extras (salary, company type); skills live
	// in Tags so the scorer doesn't see the same keyword twice.
	var rawParts []string
	for _, p := range []string{salary, companyType} {
		if p != "" {
			rawParts = append(rawParts, p)
		}
	}
	raw := strings.Join(rawParts, " ")

	return Job{
		Source:   "programathor",
		ID:       id,
		URL:      "https://programathor.com.br" + href,
		Title:    title,
		Company:  company,
		Location: location,
		Remote:   isRemote,
		Salary:   salary,
		Tags:     tags,
		Raw:      raw,
	}
}

// programathorDetail fetches a job detail page and extracts the salary range
// and the description (Atividades e Responsabilidades + Requisitos). Called
// lazily (e.g. by the TUI detail panel), not during collect.
func programathorDetail(url string) (salary, description string, err error) {
	resp, err := clientGet(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("http %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", err
	}

	// Salary: "<span class=icon-offer>…</span> Salário: Até R$4.000"
	if p := doc.Find("p:contains('Salário')").First(); p.Length() > 0 {
		salary = strings.TrimPrefix(strings.TrimSpace(p.Text()), "Salário:")
		salary = strings.TrimSpace(salary)
	}

	// Description: the <h3> sections "Atividades e Responsabilidades" and
	// "Requisitos", capturing text until the next <h3>.
	var parts []string
	doc.Find(".line-height-2-4 h3").Each(func(_ int, h3 *goquery.Selection) {
		title := strings.TrimSpace(h3.Text())
		if title != "Atividades e Responsabilidades" && title != "Requisitos" {
			return
		}
		next := h3.Next()
		for next.Length() > 0 {
			if next.Is("h3") {
				break
			}
			if t := strings.TrimSpace(next.Text()); t != "" {
				parts = append(parts, t)
			}
			next = next.Next()
		}
	})
	description = strings.TrimSpace(strings.Join(parts, "\n"))
	return salary, description, nil
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

// isTagText reports whether a card span is a skill tag rather than a
// company/location/salary/level fragment.
func isTagText(text string) bool {
	t := strings.ToLower(text)
	if text == "" || len(text) >= 30 {
		return false
	}
	if strings.Contains(text, "fa-") || strings.Contains(text, "fa ") {
		return false
	}
	for _, frag := range []string{
		"remoto", "presencial", "clt", "pj", "júnior", "junior", "pleno",
		"sênior", "senior", "estágio", "estagio", "até", "startup",
		"grande empresa", "pequena", "aceito", "aceita", "não", "nao",
	} {
		if strings.Contains(t, frag) {
			return false
		}
	}
	return true
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
