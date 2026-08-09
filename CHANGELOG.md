# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Fonte `greenhouse` (boards-api.greenhouse.io, API pública sem auth) com
  lista de empresas configurável em `~/.config/tabelavagas/sources.toml`
- Removida a fonte `gupy` (token inviável sem plano Premium/Enterprise)
- TUI elaborada: cards de 3 linhas (score+título, empresa/local/tipo, tags+
  deadline), painel de detalhes (`o`), filtro live (`/` com tokens `remote`,
  `score:NN`, `src:NOME`, `tipo:...` e busca por palavra), destaque da vaga
  focada full-width no acento
- TUI roda collect + rank juntos (`c`), então vagas novas aparecem na hora
- Veto de vagas na TUI (`x`): vetadas somem da lista, marcadas `✕` (toggle
  `V`), e não entram no `top`/`notify` do CLI (coluna `vetoed`, com migração
  automática de DBs existentes)
- Barra de status (notice bar) entre corpo e footer: feedback transitório
  (collect/veto/reload) auto-limpa em 3s
- Spinner + progresso por fonte durante o collect (streaming em background)
- Visão de faixas (`t`): 3 colunas por score (80-100 · 60-79 · <60) com
  navegação horizontal `h`/`l`
- Sidebar de perfis (`ctrl+e`): lista dev/data/fullstack/custom, `enter`
  aplica o perfil e re-scoreia tudo
- Log de atividade (`L`): collect/veto/notify/perfil com timestamps

### Fixed
- `notify 10` explícito não virava 5: N passado pelo usuário agora é respeitado
- TUI mostrava e re-notificava vagas já notificadas; agora carrega só as novas
  (`topUnnotified`) e `runNotify` não abre segunda conexão ao SQLite
- `--min N` agora filtra de verdade o `top`/`notify` (antes não fazia nada)
- Programathor: company/location/salary agora saem do ícone Font Awesome
  (classe do `<i>`), não do texto do span — antes tudo caía vazio
- Cache LLM: `jobHash` inclui `Remote`/`Type`/`Deadline` (evita score stale)
- `Raw` não duplica mais as tags no texto de scoring

## [0.1.0] - 2026-08-09

### Added
- CLI (`collect`, `sources`, `rank`, `top`, `notify`, `all`, default TUI estático)
- Coleta multi-fonte com distinção explícita `api` vs `scraping` por fonte:
  - `remotive` (API pública real, sem auth)
  - `programathor` (best-effort scraping; hoje SPAs)
- Score heurístico 0-100 configurável por perfil (`score.go`)
- Perfis custom via `~/.config/tabelavagas/profiles.toml` (sobrescrevem built-ins)
- Store SQLite com dedup por `(source, id)`
- Rastreio de notificação: vagas enviadas ficam marcadas; `--only-new` só mostra as novas
- Notificação via DMS (desktop notification) com fallback para stdout sem `dms`
