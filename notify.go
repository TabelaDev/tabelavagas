package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// notifyJobs sends the top-ranked jobs to the desktop via DMS notification.
// When DMS is not available, it prints to stdout (dry-run).
func notifyJobs(jobs []Job) {
	if len(jobs) == 0 {
		fmt.Fprintln(stdout(), "notify: sem vagas novas de alto score")
		return
	}

	// Check if the notifier is available
	if _, err := exec.LookPath(settings.Notify.Binary); err != nil {
		// Fallback: print to stdout
		fmt.Fprintf(stdout(), "=== notify (dry-run: %s não encontrado) ===\n", settings.Notify.Binary)
		for _, j := range jobs {
			fmt.Fprintf(stdout(), "[%02d] %s — %s %s\n%s\n\n", j.Score, j.Title, j.Company, j.Location, j.URL)
		}
		return
	}

	// Build notification body
	var lines []string
	for _, j := range jobs {
		loc := j.Location
		if loc == "" {
			loc = "—"
		}
		lines = append(lines, fmt.Sprintf("%d. %s\n   %s · %s\n   %s", j.Score, j.Title, j.Company, loc, j.URL))
	}

	summary := fmt.Sprintf("tabelhavagas — %d vagas que valem a pena", len(jobs))
	body := strings.Join(lines, "\n\n")

	// Send via the configured notifier
	cmd := exec.Command(settings.Notify.Binary, "notify", summary, body,
		"--app", settings.Notify.AppName,
		"--timeout", strconv.Itoa(settings.Notify.TimeoutMS),
	)
	cmd.Stdout = stdout()
	cmd.Stderr = stderr()
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr(), "aviso: %s notify falhou: %v\n", settings.Notify.Binary, err)
		// Fallback to stdout
		fmt.Fprintln(stdout(), "=== notify (fallback) ===")
		for _, j := range jobs {
			fmt.Fprintf(stdout(), "[%02d] %s — %s %s\n%s\n\n", j.Score, j.Title, j.Company, j.Location, j.URL)
		}
		return
	}
	// Only mark as notified when actually delivered via DMS, so --only-new
	// keeps the ones that were skipped.
	markJobsNotified(jobs)
	fmt.Fprintf(stdout(), "notify: %d vagas enviadas via DMS\n", len(jobs))
}

// markJobsNotified flags the delivered jobs so future --only-new runs skip them.
func markJobsNotified(jobs []Job) {
	store, err := openStore()
	if err != nil {
		return
	}
	defer store.close()
	if err := store.markNotified(jobs); err != nil {
		fmt.Fprintf(stderr(), "aviso: falha ao marcar vagas notificadas: %v\n", err)
	}
}
