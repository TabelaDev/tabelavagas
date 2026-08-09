# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- CLI (`collect`, `sources`, `rank`, `top`, `notify`, `all`, default TUI estático)
- Coleta multi-fonte com distinção explícita `api` vs `scraping` por fonte:
  - `remotive` (API pública real, sem auth)
  - `gupy` (API oficial com Bearer token via env)
  - `programathor` / `trampos` (best-effort scraping; hoje SPAs)
- Score heurístico 0-100 configurável por perfil (`score.go`)
- Perfis custom via `~/.config/tabelavagas/profiles.toml` (sobrescrevem built-ins)
- Store SQLite com dedup por `(source, id)`
- Rastreio de notificação: vagas enviadas ficam marcadas; `--only-new` só mostra as novas
- Notificação via DMS (desktop notification) com fallback para stdout sem `dms`
