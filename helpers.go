package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func get(rawURL string) (*http.Response, error) {
	return httpClient.Get(rawURL)
}

func readAll(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

func printJobs(jobs []Job) {
	if len(jobs) == 0 {
		fmt.Fprintln(stdout(), "nenhuma vaga salva ainda. rode: tvagas collect && tvagas rank")
		return
	}
	for _, j := range jobs {
		loc := j.Location
		if loc == "" {
			loc = "—"
		}
		remote := ""
		if j.Remote {
			remote = " · remoto"
		}
		fmt.Fprintf(stdout(), "[%3d] %s — %s (%s%s)\n      %s\n", j.Score, j.Title, j.Company, loc, remote, j.URL)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
