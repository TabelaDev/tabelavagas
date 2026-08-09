package main

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	headerLines = 1
	footerLines = 1
	spinnerW    = 2
)

type mode int

const (
	modeList mode = iota
	modeCollecting
)

type model struct {
	jobs    []Job
	cursor  int
	total   int
	width   int
	height  int
	mode    mode
	status  string
	profile string
}

type jobsLoadedMsg struct {
	jobs []Job
}

type collectDoneMsg struct {
	n   int
	err error
}

func newModel() model {
	return model{
		profile: defaultProfile(),
	}
}

func (m model) Init() tea.Cmd {
	return m.loadJobs
}

func (m model) loadJobs() tea.Msg {
	store, err := openStore()
	if err != nil {
		return jobsLoadedMsg{nil}
	}
	defer store.close()
	jobs, err := store.top(50)
	if err != nil {
		return jobsLoadedMsg{nil}
	}
	return jobsLoadedMsg{jobs}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case jobsLoadedMsg:
		m.jobs = msg.jobs
		m.total = len(msg.jobs)
		m.mode = modeList
		if m.total == 0 {
			m.status = "base vazia. pressione c para coletar"
		} else {
			m.status = ""
		}
		return m, nil

	case collectDoneMsg:
		m.mode = modeList
		if msg.err != nil {
			m.status = fmt.Sprintf("erro: %v", msg.err)
		} else {
			m.status = fmt.Sprintf("collect: %d novas vagas", msg.n)
		}
		return m, m.loadJobs

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit

	case "j", "down":
		if m.cursor < m.total-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if m.total > 0 {
			m.cursor = m.total - 1
		}
	case "pgup":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "pgdown":
		m.cursor += 10
		if m.cursor >= m.total {
			m.cursor = m.total - 1
		}

	case "enter":
		if m.total > 0 && m.cursor < m.total {
			url := m.jobs[m.cursor].URL
			return m, openURL(url)
		}

	case "c":
		m.mode = modeCollecting
		m.status = "coletando..."
		return m, m.runCollect

	case "r":
		return m, m.loadJobs

	case "n":
		if m.total > 0 {
			return m, m.runNotify
		}
	}
	return m, nil
}

func (m model) runCollect() tea.Msg {
	n, err := runCollect()
	return collectDoneMsg{n: n, err: err}
}

func (m model) runNotify() tea.Msg {
	store, err := openStore()
	if err != nil {
		return collectDoneMsg{err: err}
	}
	defer store.close()
	n := 5
	if m.total < n {
		n = m.total
	}
	jobs := m.jobs[:n]
	notifyJobs(jobs)
	return collectDoneMsg{n: len(jobs)}
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("xdg-open", url)
		cmd.Start()
		return nil
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "carregando..."
	}

	w := m.width

	// Header
	header := headerStyle(w).Render("tabelavagas")

	// Content area
	availH := m.height - headerLines - footerLines
	if availH < 1 {
		availH = 1
	}

	var content string
	if m.mode == modeCollecting {
		content = padToHeight(" coletando vagas...", availH)
	} else if m.total == 0 {
		content = padToHeight(" nenhuma vaga encontrada. pressione c para coletar", availH)
	} else {
		content = m.renderJobs(w, availH)
	}

	// Footer
	footer := m.renderFooter(w)
	footerBar := footerStyle(w).Render(footer)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footerBar)
}

func (m model) renderJobs(w, availH int) string {
	// Calculate visible window
	visibleRows := availH
	if visibleRows > m.total {
		visibleRows = m.total
	}

	// Scroll to keep cursor visible
	start := m.cursor - visibleRows/2
	if start < 0 {
		start = 0
	}
	if start+visibleRows > m.total {
		start = m.total - visibleRows
	}
	if start < 0 {
		start = 0
	}

	var lines []string
	for i := start; i < start+visibleRows && i < m.total; i++ {
		lines = append(lines, m.renderJob(i, w))
	}

	return padToHeight(strings.Join(lines, "\n"), availH)
}

func (m model) renderJob(i, w int) string {
	j := m.jobs[i]
	isCursor := i == m.cursor

	// Score badge
	scoreStr := fmt.Sprintf("[%3d]", j.Score)
	var score string
	if j.Score >= 80 {
		score = successStyle().Render(scoreStr)
	} else if j.Score >= 60 {
		score = warningStyle().Render(scoreStr)
	} else {
		score = dimStyle().Render(scoreStr)
	}

	// Title + company
	title := j.Title
	company := j.Company
	if company == "" {
		company = "—"
	}

	// Location
	loc := j.Location
	if loc == "" {
		loc = "—"
	}
	remote := ""
	if j.Remote {
		remote = " · remoto"
	}

	// Build line
	line := fmt.Sprintf("%s %s — %s (%s%s)", score, title, company, loc, remote)

	// Truncate to width
	if w > 0 {
		runes := []rune(line)
		if len(runes) > w {
			line = string(runes[:w-1]) + "…"
		}
	}

	if isCursor {
		return panelStyle(true).Render(line)
	}
	return line
}

func (m model) renderFooter(w int) string {
	profileStr := m.profile
	if m.total > 0 {
		return fmt.Sprintf(" %d vagas · perfil: %s · j/k navegar · enter abrir · c coletar · n notificar · q sair", m.total, profileStr)
	}
	return fmt.Sprintf(" perfil: %s · c coletar · q sair", profileStr)
}
