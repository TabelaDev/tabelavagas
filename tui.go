package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ianptkcs/tabelatuiui"
)

const (
	headerLines   = 1
	gapLines      = 1
	footerLines   = 1
	noticeLines   = 1
	cardH         = 4
	detailMin     = 32
	detailMax     = 64
	sidebarW      = 22
	maxLoad       = 1000
	noticeTimeout = 3 * time.Second
	maxLog        = 100
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type mode int

const (
	modeList mode = iota
	modeCollecting
	modeFilter
	modeLogs
	modeLLM
	modeConfirm
)

type logEntry struct {
	at   time.Time
	kind string
	text string
}

type model struct {
	all           []Job // full loaded set (unfiltered)
	jobs          []Job // filtered view
	cursor        int
	total         int
	width         int
	height        int
	mode          mode
	status        string
	statusTemp    bool
	statusGen     int
	profile       string
	showDetail    bool
	showVetoed    bool
	filter        string
	filterCur     int
	spinner       int
	collectLines  []string
	collectCh     chan string
	sidebar       bool
	sidebarIdx    int
	profiles      []string
	tierView      bool
	colIdx        int
	log           []logEntry
	llmCh         chan llmScoreMsg
	llmDone       int
	llmTotal      int
	llmJobs       []Job
	confirmText   string
	detailLoading string // "source:id" of the job whose detail is being fetched

	// helpModal is the "?" overlay listing every keybinding; settingsModal is
	// the "," overlay that lets the user rebind them. Both read from reg.
	helpModal     *tuiui.HelpModal
	settingsModal *tuiui.SettingsModal
}

type jobsLoadedMsg struct {
	jobs []Job
}

type collectDoneMsg struct {
	kind   string // "collect", "notify", "erro"
	n      int
	err    error
	notice string
}

type collectProgressMsg struct {
	line string
}

type collectTickMsg struct{}

type noticeTickMsg struct {
	gen int
}

type vetoDoneMsg struct {
	source, id, title string
	vetoed            bool
	err               error
}

type profileAppliedMsg struct {
	name string
	err  error
}

// llmScoreMsg reports LLM scoring progress or its final result. jobs != nil
// means scoring finished; otherwise done/total is progress.
type llmScoreMsg struct {
	done, total int
	jobs        []Job
	err         error
}

type llmTickMsg struct{}

// detailLoadedMsg carries the lazily-fetched job detail (salary/description).
type detailLoadedMsg struct {
	source, id, salary, description string
	err                             error
}

// activityLogMsg carries the persisted activity history loaded from the DB.
type activityLogMsg struct {
	entries []logEntry
}

func newModel() model {
	profiles := profileNames()
	idx := 0
	for i, p := range profiles {
		if p == defaultProfile() {
			idx = i
			break
		}
	}
	_ = reg.Load()
	return model{
		profile:    defaultProfile(),
		profiles:   profiles,
		sidebarIdx: idx,
		helpModal: tuiui.NewHelpModal(
			tuiui.HelpSection{
				Title: "Navegação",
				BindingsFn: func() []tuiui.Binding {
					return bindingsOf("move-down", "move-up", "move-left", "move-right", "top", "bottom", "page-up", "page-down", "open")
				},
			},
			tuiui.HelpSection{
				Title: "Ações",
				BindingsFn: func() []tuiui.Binding {
					return bindingsOf("filter", "detail", "tiers", "sidebar", "veto", "show-veto", "collect", "notify", "llm", "logs", "refresh")
				},
			},
			tuiui.HelpSection{
				Title:      "Sessão",
				BindingsFn: func() []tuiui.Binding { return bindingsOf("help", "settings", "quit") },
			},
		),
		settingsModal: tuiui.NewSettingsModal(reg),
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
	// Only unnotified jobs (the TUI is the "daily" view). Vetoed jobs are
	// loaded too so the V toggle can reveal them without a reload.
	jobs, err := store.topFiltered(maxLoad, 0, true, true)
	if err != nil {
		return jobsLoadedMsg{nil}
	}
	return jobsLoadedMsg{jobs}
}

// loadActivityLog loads the persisted activity history (up to 7 days) from
// the DB when the log view opens.
func (m model) loadActivityLog() tea.Msg {
	store, err := openStore()
	if err != nil {
		return activityLogMsg{}
	}
	defer store.close()
	acts, err := store.recentActivity(maxLog)
	if err != nil {
		return activityLogMsg{}
	}
	entries := make([]logEntry, len(acts))
	for i, a := range acts {
		entries[i] = logEntry{at: a.At, kind: a.Kind, text: a.Detail}
	}
	return activityLogMsg{entries: entries}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.helpModal.SetSize(msg.Width, msg.Height)
		m.settingsModal.SetSize(msg.Width, msg.Height)
		return m, nil

	case jobsLoadedMsg:
		m.all = msg.jobs
		m.applyFilter()
		m.mode = modeList
		if m.total == 0 && m.filter == "" {
			return m.setStatus("base vazia. pressione c para coletar", false)
		}
		return m, nil

	case collectDoneMsg:
		m.mode = modeList
		m.collectLines = nil
		m.collectCh = nil
		if msg.err != nil {
			m = m.appendLog("erro", "erro: "+msg.err.Error())
			return m.setStatus("erro: "+msg.err.Error(), false)
		}
		m = m.appendLog(msg.kind, msg.notice)
		nm, sc := m.setStatus(msg.notice, true)
		return nm, tea.Batch(sc, nm.loadJobs)

	case collectProgressMsg:
		m.collectLines = append(m.collectLines, msg.line)
		m.spinner++
		return m, m.collectDriver

	case collectTickMsg:
		m.spinner++
		return m, m.collectDriver

	case noticeTickMsg:
		if msg.gen == m.statusGen && m.statusTemp {
			m.status = ""
		}
		return m, nil

	case vetoDoneMsg:
		if msg.err != nil {
			m = m.appendLog("erro", "erro: "+msg.err.Error())
			return m.setStatus("erro: "+msg.err.Error(), false)
		}
		for i := range m.all {
			if m.all[i].Source == msg.source && m.all[i].ID == msg.id {
				m.all[i].Vetoed = msg.vetoed
				break
			}
		}
		m.applyFilter()
		verb := "vetada"
		if !msg.vetoed {
			verb = "desvetada"
		}
		m = m.appendLog("veto", verb+": "+msg.title)
		return m.setStatus(verb+": "+msg.title, true)

	case profileAppliedMsg:
		if msg.err != nil {
			m = m.appendLog("erro", "erro: "+msg.err.Error())
			return m.setStatus("erro: "+msg.err.Error(), false)
		}
		m.profile = msg.name
		m.sidebar = false
		m = m.appendLog("profile", "perfil: "+msg.name)
		nm, sc := m.setStatus("perfil: "+msg.name+" · re-scoreado", true)
		return nm, tea.Batch(sc, nm.loadJobs)

	case llmScoreMsg:
		if msg.err != nil {
			m.mode = modeList
			m.llmCh = nil
			return m.setStatus("erro: "+msg.err.Error(), false)
		}
		if msg.jobs != nil {
			m.mode = modeConfirm
			m.llmCh = nil
			m.llmJobs = msg.jobs
			m.confirmText = fmt.Sprintf("aplicar scores LLM em %d vagas?", len(msg.jobs))
			return m, nil
		}
		m.llmDone, m.llmTotal = msg.done, msg.total
		m.spinner++
		return m, m.llmDriver

	case llmTickMsg:
		m.spinner++
		return m, m.llmDriver

	case detailLoadedMsg:
		m.detailLoading = ""
		if msg.err != nil {
			return m.setStatus("erro ao buscar descrição: "+msg.err.Error(), true)
		}
		if store, err := openStore(); err == nil {
			_ = store.setDetails(msg.source, msg.id, msg.salary, msg.description)
			store.close()
		}
		for i := range m.all {
			if m.all[i].Source == msg.source && m.all[i].ID == msg.id {
				m.all[i].Salary = msg.salary
				m.all[i].Description = msg.description
			}
		}
		for i := range m.jobs {
			if m.jobs[i].Source == msg.source && m.jobs[i].ID == msg.id {
				m.jobs[i].Salary = msg.salary
				m.jobs[i].Description = msg.description
			}
		}
		return m.setStatus("descrição carregada", true)

	case activityLogMsg:
		m.log = msg.entries
		return m, nil

	case tea.KeyMsg:
		// The settings/help modals swallow all keys while open — the app
		// must not act on them (so "q" closes the modal instead of quitting).
		if m.settingsModal.Update(msg) {
			return m, nil
		}
		if m.helpModal.Update(msg) {
			return m, nil
		}
		nm, cmd := m.handleKey(msg)
		nm2, fetchCmd := nm.(model).maybeFetchDetail()
		if fetchCmd != nil {
			cmd = tea.Batch(cmd, fetchCmd)
		}
		return nm2, cmd
	}
	return m, nil
}

// appendLog records an activity persistently (7-day window in the DB) and in
// memory (capped at maxLog for the current session).
func (m model) appendLog(kind, text string) model {
	persistActivity(kind, text)
	m.log = append(m.log, logEntry{at: time.Now(), kind: kind, text: text})
	if len(m.log) > maxLog {
		m.log = m.log[len(m.log)-maxLog:]
	}
	return m
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeLogs:
		return m.handleLogKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	default:
		return m.handleListKey(msg)
	}
}

func (m model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.sidebar {
		return m.handleSidebarKey(msg)
	}
	return m.handleBodyKey(msg)
}

func (m model) handleBodyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, resolve("quit")):
		return m, tea.Quit
	case key.Matches(msg, resolve("help")):
		m.helpModal.Toggle()
		return m, nil
	case key.Matches(msg, resolve("settings")):
		m.settingsModal.Toggle()
		return m, nil
	case key.Matches(msg, resolve("sidebar")):
		m.sidebar = true
	case key.Matches(msg, resolve("tiers")):
		m.tierView = !m.tierView
		if m.tierView {
			m.clampTierCursor()
		} else if m.cursor >= m.total {
			m.cursor = m.total - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
	case key.Matches(msg, resolve("logs")):
		m.mode = modeLogs
		return m, m.loadActivityLog
	case key.Matches(msg, resolve("filter")):
		m.mode = modeFilter
		m.filterCur = len(m.filter)
	case key.Matches(msg, resolve("detail")):
		if !m.tierView {
			m.showDetail = !m.showDetail
		}
	case key.Matches(msg, resolve("veto")):
		if j, ok := m.focusedJob(); ok {
			return m, func() tea.Msg { return m.runVetoToggle(j) }
		}
	case key.Matches(msg, resolve("show-veto")):
		m.showVetoed = !m.showVetoed
		m.applyFilter()
		if m.showVetoed {
			return m.setStatus("mostrando vagas vetadas", true)
		}
		return m.setStatus("escondendo vagas vetadas", true)
	case key.Matches(msg, resolve("move-down")):
		if m.tierView {
			m.cursor++
			m.clampTierCursor()
		} else if m.cursor < m.total-1 {
			m.cursor++
		}
	case key.Matches(msg, resolve("move-up")):
		if m.tierView {
			m.cursor--
			m.clampTierCursor()
		} else if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, resolve("move-left")):
		if m.tierView && m.colIdx > 0 {
			m.colIdx--
			m.clampTierCursor()
		}
	case key.Matches(msg, resolve("move-right")):
		if m.tierView && m.colIdx < 2 {
			m.colIdx++
			m.clampTierCursor()
		}
	case key.Matches(msg, resolve("top")):
		m.cursor = 0
	case key.Matches(msg, resolve("bottom")):
		if m.tierView {
			b := m.tierGroups()
			if m.colIdx < len(b) && len(b[m.colIdx].jobs) > 0 {
				m.cursor = len(b[m.colIdx].jobs) - 1
			}
		} else if m.total > 0 {
			m.cursor = m.total - 1
		}
	case key.Matches(msg, resolve("page-up")):
		m.cursor -= 10
		m.clampCursor()
	case key.Matches(msg, resolve("page-down")):
		m.cursor += 10
		m.clampCursor()
	case key.Matches(msg, resolve("open")):
		if j, ok := m.focusedJob(); ok {
			m = m.appendLog("open", j.Title)
			return m, openURL(j.URL)
		}
	case key.Matches(msg, resolve("collect")):
		m.mode = modeCollecting
		m.spinner = 0
		m.collectLines = nil
		m.status = ""
		m.collectCh = make(chan string, 16)
		go m.collectStream()
		return m, m.collectDriver
	case key.Matches(msg, resolve("refresh")):
		nm, c := m.setStatus("recarregado", true)
		return nm, tea.Batch(c, m.loadJobs)
	case key.Matches(msg, resolve("notify")):
		if _, ok := m.focusedJob(); ok {
			return m, m.runNotify
		}
	case key.Matches(msg, resolve("llm")):
		if os.Getenv("TABELAVAGAS_LLM_API_KEY") == "" {
			return m.setStatus("chave LLM não definida (TABELAVAGAS_LLM_API_KEY)", false)
		}
		m.mode = modeLLM
		m.spinner = 0
		m.llmDone, m.llmTotal = 0, 0
		m.status = ""
		m.llmCh = make(chan llmScoreMsg, 64)
		go m.llmScoreStream()
		return m, m.llmDriver
	}
	return m, nil
}

func (m model) handleSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.helpModal.Toggle()
		return m, nil
	case ",":
		m.settingsModal.Toggle()
		return m, nil
	case "ctrl+e", "esc", "l", "right":
		m.sidebar = false
	case "j", "down":
		if m.sidebarIdx < len(m.profiles)-1 {
			m.sidebarIdx++
		}
	case "k", "up":
		if m.sidebarIdx > 0 {
			m.sidebarIdx--
		}
	case "g":
		m.sidebarIdx = 0
	case "G":
		m.sidebarIdx = len(m.profiles) - 1
	case "enter":
		if len(m.profiles) > 0 {
			return m, func() tea.Msg { return m.runApplyProfile(m.profiles[m.sidebarIdx]) }
		}
	}
	return m, nil
}

func (m model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "L", "ctrl+c":
		m.mode = modeList
	}
	return m, nil
}

// handleConfirmKey resolves the y/N decision after LLM scoring: y applies the
// cached LLM scores to the active score column, anything else keeps heuristic.
func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "y", "Y":
		store, err := openStore()
		if err != nil {
			m.mode = modeList
			return m.setStatus("erro: "+err.Error(), false)
		}
		defer store.close()
		m.mode = modeList
		jobs := m.llmJobs
		m.llmJobs = nil
		if err := store.applyScores(jobs); err != nil {
			return m.setStatus("erro: "+err.Error(), false)
		}
		m = m.appendLog("llm", "scores LLM aplicados")
		nm, sc := m.setStatus("scores LLM aplicados", true)
		return nm, tea.Batch(sc, nm.loadJobs)
	case "n", "N", "esc", "q":
		m.mode = modeList
		m.llmJobs = nil
		m = m.appendLog("llm", "scores LLM mantidos em cache")
		return m.setStatus("scores LLM mantidos em cache (score atual intacto)", true)
	}
	return m, nil
}

func (m model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.mode = modeList
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyBackspace:
		if m.filterCur > 0 {
			m.filter = m.filter[:m.filterCur-1] + m.filter[m.filterCur:]
			m.filterCur--
			m.applyFilter()
		}
	case tea.KeyLeft:
		if m.filterCur > 0 {
			m.filterCur--
		}
	case tea.KeyRight:
		if m.filterCur < len(m.filter) {
			m.filterCur++
		}
	case tea.KeyHome:
		m.filterCur = 0
	case tea.KeyEnd:
		m.filterCur = len(m.filter)
	case tea.KeyCtrlU:
		m.filter = ""
		m.filterCur = 0
		m.applyFilter()
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.filter = m.filter[:m.filterCur] + string(r) + m.filter[m.filterCur:]
			m.filterCur++
		}
		m.applyFilter()
	}
	return m, nil
}

// setStatus sets the notice bar content; temp messages auto-clear after
// noticeTimeout via a generation-guarded tick.
func (m model) setStatus(s string, temp bool) (model, tea.Cmd) {
	m.status = s
	m.statusTemp = temp
	m.statusGen++
	if temp {
		gen := m.statusGen
		return m, tea.Tick(noticeTimeout, func(time.Time) tea.Msg {
			return noticeTickMsg{gen: gen}
		})
	}
	return m, nil
}

// applyFilter recomputes the visible jobs from the full set and keeps the
// cursor in bounds.
func (m *model) applyFilter() {
	f := parseFilter(m.filter)
	filtered := make([]Job, 0, len(m.all))
	for _, j := range m.all {
		if j.Vetoed && !m.showVetoed {
			continue
		}
		if f.matches(j) {
			filtered = append(filtered, j)
		}
	}
	m.jobs = filtered
	m.total = len(filtered)
	m.clampCursor()
}

// clampCursor keeps the list cursor (and tier column) within bounds.
func (m *model) clampCursor() {
	if m.tierView {
		m.clampTierCursor()
		return
	}
	if m.cursor >= m.total {
		m.cursor = m.total - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// focusedJob returns the job under the cursor, in either list or tier view.
func (m model) focusedJob() (Job, bool) {
	if m.tierView {
		return m.focusedTierJob()
	}
	if m.total > 0 && m.cursor < m.total {
		return m.jobs[m.cursor], true
	}
	return Job{}, false
}

// maybeFetchDetail lazily fetches the job detail page (salary/description)
// when the detail panel is showing a Programathor job without a description
// yet. Runs once per job (guarded by detailLoading).
func (m model) maybeFetchDetail() (model, tea.Cmd) {
	if m.mode != modeList || !m.showDetail || m.detailLoading != "" {
		return m, nil
	}
	j, ok := m.focusedJob()
	if !ok || j.Source != "programathor" || j.Description != "" {
		return m, nil
	}
	m.detailLoading = j.Source + ":" + j.ID
	return m, func() tea.Msg {
		salary, desc, err := programathorDetail(j.URL)
		return detailLoadedMsg{source: j.Source, id: j.ID, salary: salary, description: desc, err: err}
	}
}

// tiers partitions the visible jobs into three score brackets.
func (m model) tierGroups() []bracket {
	b := []bracket{
		{label: "80-100"},
		{label: "60-79"},
		{label: "<60"},
	}
	for _, j := range m.jobs {
		switch {
		case j.Score >= 80:
			b[0].jobs = append(b[0].jobs, j)
		case j.Score >= 60:
			b[1].jobs = append(b[1].jobs, j)
		default:
			b[2].jobs = append(b[2].jobs, j)
		}
	}
	return b
}

func (m model) focusedTierJob() (Job, bool) {
	b := m.tierGroups()
	if m.colIdx < 0 || m.colIdx >= len(b) || m.cursor >= len(b[m.colIdx].jobs) {
		return Job{}, false
	}
	return b[m.colIdx].jobs[m.cursor], true
}

func (m *model) clampTierCursor() {
	b := m.tierGroups()
	if m.colIdx < 0 || m.colIdx >= len(b) {
		return
	}
	n := len(b[m.colIdx].jobs)
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m model) runVetoToggle(j Job) tea.Msg {
	store, err := openStore()
	if err != nil {
		return vetoDoneMsg{err: err}
	}
	defer store.close()
	v := !j.Vetoed
	if err := store.setVetoed(j.Source, j.ID, v); err != nil {
		return vetoDoneMsg{err: err}
	}
	return vetoDoneMsg{source: j.Source, id: j.ID, vetoed: v, title: j.Title}
}

func (m model) runApplyProfile(name string) tea.Msg {
	if err := rescoreProfile(name); err != nil {
		return profileAppliedMsg{name: name, err: err}
	}
	return profileAppliedMsg{name: name}
}

// collectStream runs the collectors in the background and streams one line
// per source plus a final status into m.collectCh.
func (m model) collectStream() {
	ch := m.collectCh
	// Per-source progress already goes through the callback, but loading the
	// sources and profile config warns straight to the sinks on a malformed
	// TOML — which would print over the alt-screen.
	restore := captureOutput()
	defer func() {
		if out := restore(); out != "" {
			persistActivity("erro", firstLine(out))
		}
	}()

	total, err := runCollectProgress(func(source string, added int, cerr error) {
		if cerr != nil {
			ch <- "aviso " + source + ": " + cerr.Error()
		} else {
			ch <- fmt.Sprintf("%s: %d novas", source, added)
		}
	})
	if err != nil {
		ch <- "ERR: " + err.Error()
		close(ch)
		return
	}
	store, err := openStore()
	if err == nil {
		sc := buildScorer(cmdFlags{profile: m.profile}, store)
		if err := scoreAll(store, sc); err != nil {
			ch <- "aviso: rank falhou"
			ch <- "DONE:" + strconv.Itoa(total) + ":fail"
		} else {
			ch <- "rank ok"
			ch <- "DONE:" + strconv.Itoa(total) + ":ok"
		}
		store.close()
	} else {
		ch <- "aviso: rank falhou"
		ch <- "DONE:" + strconv.Itoa(total) + ":fail"
	}
	close(ch)
}

// collectDriver polls the collect channel, surfacing progress lines and the
// spinner frame; re-scheduled until it sees the DONE marker.
func (m model) collectDriver() tea.Msg {
	select {
	case line, ok := <-m.collectCh:
		if !ok {
			return collectDoneMsg{notice: "collect encerrado"}
		}
		switch {
		case strings.HasPrefix(line, "DONE:"):
			rest := strings.TrimPrefix(line, "DONE:")
			parts := strings.Split(rest, ":")
			n, _ := strconv.Atoi(parts[0])
			ok := len(parts) < 2 || parts[1] != "fail"
			notice := fmt.Sprintf("collect: %d novas", n)
			if ok {
				notice += " · rank ok"
			} else {
				notice += " · rank falhou"
			}
			return collectDoneMsg{kind: "collect", n: n, notice: notice}
		case strings.HasPrefix(line, "ERR:"):
			return collectDoneMsg{kind: "erro", err: errors.New(strings.TrimPrefix(line, "ERR:"))}
		default:
			return collectProgressMsg{line: line}
		}
	case <-time.After(100 * time.Millisecond):
		return collectTickMsg{}
	}
}

// llmScoreStream scores every job with the LLM in the background, streaming
// per-job progress and the final results (with the LLM cache filled) into
// m.llmCh — the active score column is left untouched until the user confirms.
func (m model) llmScoreStream() {
	ch := m.llmCh
	store, err := openStore()
	if err != nil {
		ch <- llmScoreMsg{err: err}
		close(ch)
		return
	}
	defer store.close()

	// The LLM scorer warns once per job that fails. With a bad API key that is
	// one line per job straight onto the alt-screen, so the output is captured
	// and summarised into the activity log instead.
	restore := captureOutput()

	sc := buildScorer(cmdFlags{profile: m.profile, scorer: "llm"}, store)
	jobs, err := scoreAllLLMProgress(store, sc, func(done, total int) {
		ch <- llmScoreMsg{done: done, total: total}
	})

	if warnings := restore(); warnings != "" {
		persistActivity("erro", fmt.Sprintf("llm: %d aviso(s) — %s",
			strings.Count(warnings, "\n")+1, firstLine(warnings)))
	}

	if err != nil {
		ch <- llmScoreMsg{err: err}
	} else {
		ch <- llmScoreMsg{jobs: jobs}
	}
	close(ch)
}

// llmDriver polls the LLM channel, surfacing progress and the spinner frame;
// re-scheduled until it receives the final result.
func (m model) llmDriver() tea.Msg {
	select {
	case msg, ok := <-m.llmCh:
		if !ok {
			return llmScoreMsg{err: errors.New("pontuação LLM encerrada")}
		}
		return msg
	case <-time.After(100 * time.Millisecond):
		return llmTickMsg{}
	}
}

func (m model) runNotify() tea.Msg {
	var jobs []Job
	if m.tierView {
		col := m.tierGroups()[m.colIdx].jobs
		if len(col) > 5 {
			col = col[:5]
		}
		jobs = col
	} else {
		n := 5
		if m.total < n {
			n = m.total
		}
		jobs = m.jobs[:n]
	}
	if len(jobs) == 0 {
		return collectDoneMsg{kind: "notify", notice: "notify: nada a notificar"}
	}
	// notifyJobs writes to the output sinks — the dms fallback prints the whole
	// list, and a failed dms call prints a warning. Under the alt-screen that
	// lands on top of the rendered frame, so it goes to the activity log.
	restore := captureOutput()
	notifyJobs(jobs)
	if out := restore(); out != "" {
		persistActivity("notify", firstLine(out))
	}
	return collectDoneMsg{kind: "notify", n: len(jobs), notice: fmt.Sprintf("notify: %d vagas enviadas", len(jobs))}
}

// firstLine keeps a captured warning to a single status-bar-sized line.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
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
	header := theme.Header(w).Render("tabelavagas")

	availH := m.height - headerLines - gapLines - footerLines - noticeLines
	if availH < 1 {
		availH = 1
	}

	var body string
	switch m.mode {
	case modeCollecting:
		body = m.renderCollecting(availH, w)
	case modeFilter:
		body = m.renderFilter(availH, w)
	case modeLogs:
		body = m.renderLogs(availH, w)
	case modeLLM:
		body = m.renderLLM(availH, w)
	case modeConfirm:
		body = m.renderConfirm(availH, w)
	default:
		body = m.renderBody(availH, w)
	}

	notice := m.renderNotice(w)
	footer := tuiui.NewFooter(reg.Bindings()...).
		Status(m.footerStatus()).
		Render(w, theme)

	view := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		notice,
		footer,
	)
	if m.settingsModal.Visible() {
		return m.settingsModal.View(theme)
	}
	if m.helpModal.Visible() {
		return m.helpModal.View(theme)
	}
	return view
}

func (m model) renderBody(availH, w int) string {
	sidebar := ""
	listW := w
	if m.sidebar {
		sidebar = m.renderSidebar(availH)
		listW = w - sidebarW - 1
		if listW < 1 {
			listW = 1
		}
	}
	var main string
	if m.tierView {
		main = m.renderTiers(availH, listW)
	} else {
		main = m.renderListPanel(availH, listW)
	}
	if m.sidebar {
		return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", main)
	}
	return main
}

// renderInPanel wraps content in a bordered panel of total size w×availH.
func (m model) renderInPanel(content string, availH, w int) string {
	innerW := w - 4 // border (2) + padding (2)
	innerH := availH - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	content = padLines(content, innerW)
	content = padToHeight(content, innerH)
	return theme.Panel(false).Render(content)
}

// renderListPanel renders the job list (and detail panel, when open) inside
// bordered panels, side by side.
func (m model) renderListPanel(availH, w int) string {
	if m.showDetail && m.total > 0 {
		dw := m.detailWidth(w)
		list := m.renderJobsPanel(availH, w-dw-1)
		detail := m.renderDetail(dw, availH)
		return lipgloss.JoinHorizontal(lipgloss.Top, list, " ", detail)
	}
	return m.renderJobsPanel(availH, w)
}

func (m model) renderJobsPanel(availH, w int) string {
	return m.renderInPanel(m.renderJobs(availH-2, w-4), availH, w)
}

func (m model) renderSidebar(availH int) string {
	inner := sidebarW - 4
	lines := []string{"perfis"}
	for i, name := range m.profiles {
		line := "  " + name
		if name == m.profile {
			line = "• " + name
		}
		if i == m.sidebarIdx {
			line = theme.Title().Render("▸ " + name)
		} else if name == m.profile {
			line = theme.Dim().Render("• " + name)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", "enter: aplicar", "esc: voltar")
	content := padToHeight(strings.Join(lines, "\n"), availH-2)
	return theme.Panel(true).Render(padLines(content, inner))
}

func (m model) renderTiers(availH, w int) string {
	b := m.tierGroups()
	n := len(b)
	gap := 2
	colTotal := (w - (n-1)*gap) / n
	inner := colTotal - 4
	if inner < 20 {
		inner = 20
	}
	parts := make([]string, 0, n*2-1)
	for i, br := range b {
		if i > 0 {
			parts = append(parts, " ")
		}
		parts = append(parts, m.renderTierColumn(br, i, inner, availH))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m model) renderTierColumn(br bracket, idx, inner, availH int) string {
	focused := idx == m.colIdx
	header := fmt.Sprintf("%s (%d)", br.label, len(br.jobs))
	lines := []string{header}
	visible := (availH - 2) / cardH
	if visible < 1 {
		visible = 1
	}
	if len(br.jobs) == 0 {
		lines = append(lines, "  vazio")
	} else {
		cur := m.cursor
		if cur >= len(br.jobs) {
			cur = len(br.jobs) - 1
		}
		start := cur - visible/2
		if start < 0 {
			start = 0
		}
		if start+visible > len(br.jobs) {
			start = len(br.jobs) - visible
		}
		if start < 0 {
			start = 0
		}
		for i := start; i < start+visible && i < len(br.jobs); i++ {
			sel := focused && i == cur
			lines = append(lines, m.renderCardJob(br.jobs[i], sel, inner))
		}
	}
	content := padToHeight(strings.Join(lines, "\n"), availH-2)
	return theme.Panel(focused).Render(padLines(content, inner))
}

func (m model) renderFilter(availH, w int) string {
	inner := w - 1 // left accent border
	if inner < 10 {
		inner = 10
	}
	var left string
	if m.filter == "" {
		left = filterPromptStyle().Render("⌕ ") + theme.Dim().Render("palavras · remote · score:NN · src:NOME")
	} else {
		left = filterPromptStyle().Render("⌕ ") + filterQueryStyle().Render(m.filterWithCursor())
	}
	count := m.filterCountText()
	free := inner - lipgloss.Width(left) - lipgloss.Width(count)
	if free < 1 {
		free = 1
	}
	bar := filterBarStyle(w).Render(padLines(left+strings.Repeat(" ", free)+count, inner))
	listH := availH - 2 // bar + gap
	if listH < 1 {
		listH = 1
	}
	return bar + "\n\n" + m.renderListPanel(listH, w)
}

// filterCountText is the live match count shown at the right edge of the
// filter bar (red when nothing matches).
func (m model) filterCountText() string {
	if m.total == 0 && m.filter != "" {
		return errorStyle().Render("0 vagas")
	}
	if m.filter != "" {
		return theme.Dim().Render(fmt.Sprintf("%d/%d vagas", m.total, len(m.all)))
	}
	return theme.Dim().Render(fmt.Sprintf("%d vagas", m.total))
}

func (m model) filterWithCursor() string {
	cur := m.filterCur
	if cur < 0 {
		cur = 0
	}
	if cur > len(m.filter) {
		cur = len(m.filter)
	}
	return m.filter[:cur] + "▮" + m.filter[cur:]
}

func (m model) renderCollecting(availH, w int) string {
	frame := spinnerFrames[m.spinner%len(spinnerFrames)]
	lines := append([]string{" " + frame + " coletando vagas..."}, m.collectLines...)
	return m.renderInPanel(strings.Join(lines, "\n"), availH, w)
}

func (m model) renderNotice(w int) string {
	return mutedStyle().Render(padLines(" "+m.status, w))
}

func (m model) renderLLM(availH, w int) string {
	frame := spinnerFrames[m.spinner%len(spinnerFrames)]
	var line string
	if m.llmTotal > 0 {
		line = fmt.Sprintf(" %s pontuando com LLM... %d/%d (cache nasce aqui)", frame, m.llmDone, m.llmTotal)
	} else {
		line = fmt.Sprintf(" %s pontuando com LLM...", frame)
	}
	return m.renderInPanel(line, availH, w)
}

func (m model) renderConfirm(availH, w int) string {
	box := theme.Modal().Render(m.confirmText + "\n\n  [y] aplicar    [n] descartar (fica em cache)")
	return lipgloss.Place(w, availH, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderLogs(availH, w int) string {
	if len(m.log) == 0 {
		return m.renderInPanel(" sem atividade ainda. rode c (collect) ou n (notify)", availH, w)
	}
	inner := w - 4 // panel inner
	if inner < 1 {
		inner = 1
	}
	var sb strings.Builder
	for i := len(m.log) - 1; i >= 0; i-- {
		e := m.log[i]
		sb.WriteString(fmt.Sprintf(" %s  %s %s\n", e.at.Format("02/01 15:04"), activityBadge(e.kind), e.text))
	}
	content := wrapText(strings.TrimRight(sb.String(), "\n"), inner)
	return m.renderInPanel(content, availH, w)
}

// activityBadge renders a small colored tag for an activity kind.
func activityBadge(kind string) string {
	s := "[" + kind + "]"
	switch kind {
	case "collect":
		return successStyle().Render(s)
	case "notify":
		return infoStyle().Render(s)
	case "veto":
		return warningStyle().Render(s)
	case "llm":
		return theme.Title().Render(s)
	case "open":
		return theme.Dim().Render(s)
	case "erro":
		return errorStyle().Render(s)
	default:
		return mutedStyle().Render(s)
	}
}

func (m model) detailWidth(w int) int {
	dw := w * 40 / 100
	if dw < detailMin {
		dw = detailMin
	}
	if dw > detailMax {
		dw = detailMax
	}
	if dw >= w-1 {
		dw = w - 1
	}
	return dw
}

func (m model) renderJobs(availH, w int) string {
	if w < 1 {
		w = 1
	}
	if m.total == 0 {
		if m.filter != "" {
			return padToHeight(" nenhuma vaga casa com o filtro (esc p/ sair, ctrl+u p/ limpar)", availH)
		}
		if m.showVetoed {
			return padToHeight(" nenhuma vaga vetada.", availH)
		}
		return padToHeight(" nenhuma vaga nova. pressione c para coletar", availH)
	}
	visible := availH / cardH
	if visible < 1 {
		visible = 1
	}
	if visible > m.total {
		visible = m.total
	}
	start := m.cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > m.total {
		start = m.total - visible
	}
	blocks := make([]string, 0, visible)
	for i := start; i < start+visible && i < m.total; i++ {
		blocks = append(blocks, m.renderCard(i, w))
	}
	return padToHeight(strings.Join(blocks, "\n"), availH)
}

func (m model) renderCard(i, w int) string {
	return m.renderCardJob(m.jobs[i], i == m.cursor, w)
}

// renderCardJob draws one 3-line job block: score+title, meta line (company ·
// location · type · remoto), tags+deadline. The focused card gets a solid
// full-width accent rectangle (like tabelakanban's selected cards); unselected
// cards use plain background so colored score badges survive the ANSI stack.
func (m model) renderCardJob(j Job, sel bool, w int) string {
	title := j.Title
	if title == "" {
		title = "—"
	}
	score := fmt.Sprintf("%d", j.Score)
	marker := "▸"
	if j.Vetoed {
		marker = "✕"
	}

	var line1 string
	if sel {
		line1 = fmt.Sprintf("%s [%s] %s", marker, score, title)
	} else {
		line1 = marker + " " + m.scoreBadge(j) + " " + cardTitleStyle().Render(title)
	}

	var meta []string
	for _, p := range []string{j.Company, j.Location, j.Type} {
		if p != "" {
			meta = append(meta, p)
		}
	}
	if j.Remote {
		meta = append(meta, "remoto")
	}
	line2 := "  " + strings.Join(meta, " · ")

	line3 := "  "
	if len(j.Tags) > 0 {
		line3 += strings.Join(j.Tags, " ")
	}
	if j.Vetoed {
		if line3 != "  " {
			line3 += "   "
		}
		line3 += "vetada"
	}
	if j.Deadline != "" {
		if line3 != "  " {
			line3 += "   "
		}
		line3 += "vence: " + j.Deadline
	}

	if sel {
		s := selCardStyle()
		return lipgloss.JoinVertical(lipgloss.Left,
			s.Render(padLines(line1, w)),
			s.Render(padLines(line2, w)),
			s.Render(padLines(line3, w)),
			"", // gap between cards, not highlighted
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		padLines(line1, w),
		theme.Dim().Render(padLines(line2, w)),
		mutedStyle().Render(padLines(line3, w)),
		"", // gap between cards
	)
}

func (m model) scoreBadge(j Job) string {
	return m.scoreBadgeForScore(j.Score).Render(fmt.Sprintf("[%d]", j.Score))
}

func (m model) scoreBadgeForScore(score int) lipgloss.Style {
	switch {
	case score >= 80:
		return successStyle()
	case score >= 60:
		return warningStyle()
	default:
		return theme.Dim()
	}
}

func (m model) renderDetail(dw, availH int) string {
	j := m.jobs[m.cursor]
	inner := dw - 4 // border (2) + padding (2)
	if inner < 10 {
		inner = 10
	}

	var plain []string
	plain = append(plain, j.Title)

	var meta []string
	for _, p := range []string{j.Company, j.Location} {
		if p != "" {
			meta = append(meta, p)
		}
	}
	plain = append(plain, strings.Join(meta, " · "))

	src := j.Source
	if j.Remote {
		src += " · remoto"
	}
	detail := "Fonte: " + src
	if j.Type != "" {
		detail += " | Tipo: " + j.Type
	}
	if j.Deadline != "" {
		detail += " | Vence: " + j.Deadline
	}
	if j.Salary != "" {
		detail += " | Salário: " + j.Salary
	}
	plain = append(plain, detail)
	plain = append(plain, fmt.Sprintf("Score: %d/100", j.Score))
	plain = append(plain, "Tags: "+strings.Join(j.Tags, ", "))
	plain = append(plain, "")
	switch {
	case j.Description != "":
		plain = append(plain, j.Description)
	case j.Raw != "" && j.Source != "programathor":
		plain = append(plain, j.Raw)
	case j.Source == "programathor":
		if m.detailLoading == j.Source+":"+j.ID {
			plain = append(plain, "carregando descrição...")
		} else {
			plain = append(plain, "(descrição carrega automaticamente ao focar)")
		}
	default:
		plain = append(plain, "(sem descrição)")
	}

	ls := strings.Split(wrapText(strings.Join(plain, "\n"), inner), "\n")
	scoreStyle := m.scoreBadgeForScore(j.Score)
	for i, l := range ls {
		switch {
		case i == 0:
			ls[i] = theme.Title().Render(l)
		case i == 1:
			ls[i] = theme.Dim().Render(l)
		case strings.HasPrefix(l, "Fonte:"):
			ls[i] = theme.Dim().Render(l)
		case strings.HasPrefix(l, "Tags:"):
			ls[i] = mutedStyle().Render(l)
		case strings.HasPrefix(l, "Score:"):
			ls[i] = scoreStyle.Render(l)
		}
	}
	content := padToHeight(strings.Join(ls, "\n"), availH-2)
	return theme.Panel(true).Render(padLines(content, inner))
}

// footerStatus is the left side of the footer bar: the dynamic state summary
// (count, profile, which views are active). The right side (keybinding hints)
// is generated from reg.Bindings() by tuiui.Footer.
func (m model) footerStatus() string {
	parts := []string{fmt.Sprintf("%d vagas", m.total), "perfil: " + m.profile}
	if m.filter != "" {
		parts[0] = fmt.Sprintf("%d/%d vagas", m.total, len(m.all))
	}
	if m.showDetail {
		parts = append(parts, "detalhes")
	}
	if m.showVetoed {
		parts = append(parts, "vetadas")
	}
	if m.tierView {
		parts = append(parts, "faixas")
	}
	return strings.Join(parts, " · ")
}
