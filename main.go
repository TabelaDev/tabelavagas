package main

import (
	"fmt"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

const usage = `tabelavagas — filtra vagas que valem a pena para o teu perfil.

Uso:
  tabelavagas                  roda TUI (padrão)
  tabelavagas collect          baixa vagas das fontes e salva no SQLite
  tabelavagas sources          lista fontes e o tipo (API vs scraping)
  tabelavagas rank [flags]     score 0-100 das vagas salvas
  tabelavagas top [N] [flags]  imprime as N melhores (default 10)
  tabelavagas notify [N]       envia top N via desktop notification (DMS)
  tabelavagas all [flags]      collect → rank → top → notify

Flags comuns:
  --min N         cutoff de score (override do min_score do perfil, no scoring)
  --profile NAME  perfil de scoring (default: dev)
  --scorer TYPE   heuristic | llm (default: heuristic)
  --only-new      só vagas ainda não notificadas (top/notify/all)
  --dry           mostra o que faria sem gravar

Perfis built-in: dev, data, fullstack.
Customize em ~/.config/tabelavagas/profiles.toml (veja README).

Env:
  TABELAVAGAS_DB             path do SQLite (default ~/.local/state/tabelavagas/vagas.db)
  TABELAVAGAS_GUPY_TOKEN     bearer token da API Gupy
  TABELAVAGAS_PROFILE        perfil padrão (override de --profile)
  TABELAVAGAS_LLM_API_KEY    chave API OpenAI-compatível (DeepSeek etc.)
  TABELAVAGAS_LLM_BASEURL    base URL do provider LLM (default: api.deepseek.com/v1)
  TABELAVAGAS_LLM_MODEL      modelo LLM (default: deepseek-chat)
`

// cmdFlags holds parsed CLI flags shared by several subcommands.
type cmdFlags struct {
	profile string
	min     int
	scorer  string
	dry     bool
	topN    int
	onlyNew bool
}

func parseFlags(args []string) cmdFlags {
	f := cmdFlags{profile: defaultProfile(), min: 0, scorer: "heuristic", dry: false, topN: 10, onlyNew: false}
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--min":
			i++
			if i < len(args) {
				if v, err := strconv.Atoi(args[i]); err == nil {
					f.min = v
				}
			}
		case "--profile":
			i++
			if i < len(args) {
				f.profile = args[i]
			}
		case "--scorer":
			i++
			if i < len(args) {
				f.scorer = args[i]
			}
		case "--only-new":
			f.onlyNew = true
		case "--dry":
			f.dry = true
		default:
			// first non-flag arg is the positional (N for top/notify)
			n, err := strconv.Atoi(args[i])
			if err == nil && f.topN == 10 {
				f.topN = n
			}
		}
		i++
	}
	return f
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		runTUI()
		return
	}

	// strip flags from command args
	cmd, flagsArgs := args[0], args[1:]
	f := parseFlags(flagsArgs)

	switch cmd {
	case "sources":
		printSources()
	case "collect":
		n, err := runCollect()
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(stdout(), "collect: %d novas vagas gravadas\n", n)
	case "rank":
		if err := runRank(f); err != nil {
			fatal(err)
		}
	case "top":
		jobs, err := runTop(f)
		if err != nil {
			fatal(err)
		}
		printJobs(jobs)
	case "notify":
		n := f.topN
		if n == 10 {
			n = 5
		}
		f.topN = n
		jobs, err := runTop(f)
		if err != nil {
			fatal(err)
		}
		notifyJobs(jobs)
	case "all":
		n, err := runCollect()
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(stdout(), "collect: %d novas vagas gravadas\n", n)
		if err := runRank(f); err != nil {
			fatal(err)
		}
		jobs, err := runTop(f)
		if err != nil {
			fatal(err)
		}
		printJobs(jobs)
		notifyJobs(jobs)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tabela: erro:", err)
	os.Exit(1)
}

// buildScorer constructs a Scorer from the profile name and flags.
func buildScorer(f cmdFlags, store *Store) Scorer {
	profile := resolveProfile(f.profile)
	if f.min > 0 {
		profile.MinScore = f.min
	}
	if f.scorer == "llm" {
		// LLM scorer is implemented in llm.go; falls back to heuristic
		// if no API key is configured.
		if key := os.Getenv("TABELAVAGAS_LLM_API_KEY"); key != "" {
			return newLLMScorer(key, profile, store)
		}
		fmt.Fprintln(os.Stderr, "aviso: TABELAVAGAS_LLM_API_KEY não definida; usando heurística")
	}
	return &heuristicScorer{opts: profile}
}

func runTUI() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func runRank(f cmdFlags) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.close()
	sc := buildScorer(f, store)
	if f.dry {
		jobs, err := store.all()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout(), "dry-run: %d vagas seriam re-scoreadas com %s\n", len(jobs), f.scorer)
		for i, j := range jobs {
			if i >= 20 {
				fmt.Fprintf(stdout(), "  ... e mais %d\n", len(jobs)-20)
				break
			}
			newScore := sc.Score(j)
			delta := newScore - j.Score
			sign := "+"
			if delta < 0 {
				sign = ""
			}
			fmt.Fprintf(stdout(), "  [%3d→%3d %s%d] %s\n", j.Score, newScore, sign, delta, j.Title)
		}
		return nil
	}
	return scoreAll(store, sc)
}

func runTop(f cmdFlags) ([]Job, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	defer store.close()
	n := f.topN
	if n <= 0 {
		n = 10
	}
	if f.onlyNew {
		return store.topUnnotified(n)
	}
	return store.top(n)
}
