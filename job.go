package main

// Job is a normalized vacancy record, regardless of source.
type Job struct {
	Source      string   `json:"source"`
	ID          string   `json:"id"` // source-specific stable id (used for dedupe)
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Location    string   `json:"location"`
	Remote      bool     `json:"remote"`
	Type        string   `json:"type"` // estágio, junior, pleno, senior
	Deadline    string   `json:"deadline,omitempty"`
	Salary      string   `json:"salary,omitempty"`
	Tags        []string `json:"tags"`
	Raw         string   `json:"raw,omitempty"` // truncated description for scoring
	Description string   `json:"description,omitempty"`
	Score       int      `json:"score,omitempty"`
	Vetoed      bool     `json:"vetoed,omitempty"`
}

func (j Job) key() string {
	if j.ID != "" {
		return j.Source + ":" + j.ID
	}
	return j.Source + ":" + j.URL
}
