package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// llmScorer implements Scorer by calling an OpenAI-compatible chat completions
// endpoint. It caches scores in the DB via llm_score/llm_hash columns and
// only calls the API when the job content has changed.
type llmScorer struct {
	apiKey  string
	baseURL string
	model   string
	opts    scoreOptions
	store   *Store
}

func newLLMScorer(apiKey string, opts scoreOptions, store *Store) Scorer {
	baseURL := envOr("TABELAVAGAS_LLM_BASEURL", "https://api.deepseek.com/v1")
	model := envOr("TABELAVAGAS_LLM_MODEL", "deepseek-chat")
	return &llmScorer{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		opts:    opts,
		store:   store,
	}
}

func (l *llmScorer) Score(j Job) int {
	hash := jobHash(j)

	// Check cache if store is available
	if l.store != nil {
		if cached, cachedHash, ok := l.store.llmCachedScore(j.Source, j.ID); ok {
			if cachedHash == hash {
				return int(cached)
			}
		}
	}

	// Call LLM
	score, err := l.callLLM(j)
	if err != nil {
		fmt.Fprintf(stderr(), "aviso: LLM score falhou para %s: %v\n", j.Title, err)
		// Fallback to heuristic
		sc := &heuristicScorer{opts: l.opts}
		return sc.Score(j)
	}

	// Cache result
	if l.store != nil {
		l.store.setLLMScore(j.Source, j.ID, float64(score), hash)
	}

	return score
}

func (l *llmScorer) callLLM(j Job) (int, error) {
	prompt := buildPrompt(j)

	body := map[string]any{
		"model": l.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Você é um assistente que avalia vagas de emprego para um desenvolvedor. Responda SOMENTE com JSON: {\"score\": <0-100>, \"reason\": \"<breve razão>\"}. Score alto = boa vaga pro perfil.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.1,
		"max_tokens":  100,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	url := l.baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(b), 200))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	if len(result.Choices) == 0 {
		return 0, fmt.Errorf("no choices in response")
	}

	return parseLLMScore(result.Choices[0].Message.Content)
}

func buildPrompt(j Job) string {
	var sb strings.Builder
	sb.WriteString("Vaga de emprego:\n")
	sb.WriteString(fmt.Sprintf("Título: %s\n", j.Title))
	if j.Company != "" {
		sb.WriteString(fmt.Sprintf("Empresa: %s\n", j.Company))
	}
	if j.Location != "" {
		sb.WriteString(fmt.Sprintf("Local: %s\n", j.Location))
	}
	if j.Remote {
		sb.WriteString("Remoto: sim\n")
	}
	if len(j.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(j.Tags, ", ")))
	}
	if j.Raw != "" {
		sb.WriteString(fmt.Sprintf("Detalhes: %s\n", truncate(j.Raw, 500)))
	}
	if j.Description != "" {
		sb.WriteString(fmt.Sprintf("Descrição: %s\n", truncate(j.Description, 1000)))
	}
	sb.WriteString("\nDê um score de 0 a 100 para o quão boa é esta vaga para um desenvolvedor júnior/pleno que foca em backend, dados, ML/IA, python, go, svelte, typescript.")
	return sb.String()
}

func parseLLMScore(content string) (int, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)

	// Handle markdown code blocks
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "{") {
				content = line
				break
			}
		}
	}

	var result struct {
		Score  int    `json:"score"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Try to find score in text
		var score int
		n, _ := fmt.Sscanf(content, "%d", &score)
		if n == 1 && score >= 0 && score <= 100 {
			return score, nil
		}
		return 0, fmt.Errorf("parse LLM response: %v", err)
	}

	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}
	return result.Score, nil
}

// jobHash is the cache key for an LLM score, so it has to cover exactly the
// fields buildPrompt sends. Description and Salary were missing: setDetails
// fetches the description lazily from the source page, and when it arrived the
// hash did not change — the job kept the score computed without it, forever.
func jobHash(j Job) string {
	h := sha256.New()
	h.Write([]byte(j.Source))
	h.Write([]byte(j.ID))
	h.Write([]byte(j.Title))
	h.Write([]byte(j.Company))
	h.Write([]byte(j.Location))
	h.Write([]byte(strconv.FormatBool(j.Remote)))
	h.Write([]byte(j.Type))
	h.Write([]byte(j.Deadline))
	h.Write([]byte(j.Raw))
	h.Write([]byte(j.Description))
	h.Write([]byte(j.Salary))
	for _, t := range j.Tags {
		h.Write([]byte(t))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
