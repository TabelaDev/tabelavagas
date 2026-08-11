<div align="center">

# tabelavagas

**English** · [Português](README.pt-BR.md)

[![Go Version](https://img.shields.io/github/go-mod/go-version/TabelaDev/tabelavagas?style=flat-square&logo=go&logoColor=white&color=00ADD8)](go.mod)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue?style=flat-square)](LICENSE)
[![Built with Bubble Tea](https://img.shields.io/badge/built%20with-Bubble%20Tea-ff69b4?style=flat-square)](https://github.com/charmbracelet/bubbletea)
[![Powered by tabelatuiui](https://img.shields.io/badge/theme-tabelatuiui-d6b4f7?style=flat-square)](https://github.com/TabelaDev/tabelatuiui)

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/ianptkcs)

</div>

---

Filters programming job postings that might be worth your profile and only tells
you about the ones that matter. A Go monoband: collect → rank → top → notify.

## CLI

```
tabelavagas                  runs the TUI (default)
tabelavagas sources          lists the sources and their kind (API vs scraping)
tabelavagas collect          downloads postings from the sources into SQLite
tabelavagas rank [flags]     scores the stored postings 0-100
tabelavagas top [N] [flags]  prints the best N (10 by default)
tabelavagas notify [N]       sends the top N as a desktop notification (DMS)
tabelavagas all [flags]      collect → rank → top → notify
```

Common flags:

| Flag          | Description                                              | Default |
| ------------- | -------------------------------------------------------- | ------- |
| `--min N`     | minimum score to make `top`/`notify`                     | —       |
| `--profile X` | scoring profile (`dev`, `data`, `fullstack`)             | `dev`   |
| `--scorer X`  | `heuristic` or `llm`                                     | —       |
| `--only-new`  | only postings not notified yet (`top`/`notify`/`all`)     | —       |
| `--dry`       | shows what it would do without writing                   | —       |

## TUI

`tabelavagas` opens the TUI: a list of new postings (not notified yet), each in a
3-line card (score+title, company/location/kind, tags+deadline). The focused
posting is highlighted full-width in the accent colour.

| Key        | Action                                                          |
| ---------- | --------------------------------------------------------------- |
| `j`/`k`    | navigate (also arrows, `g`/`G`, `pgup`/`pgdn`)                  |
| `h`/`l`    | change column in the band view (`t`)                            |
| `/`        | opens the live filter — type and the list narrows               |
| `esc`      | leaves the filter (`ctrl+u` clears it)                          |
| `o`        | opens and closes the details panel                              |
| `x`        | veto/un-veto the focused posting (it leaves the list)           |
| `V`        | show/hide the vetoed postings                                   |
| `t`        | band view: columns 80-100 · 60-79 · <60                         |
| `ctrl+e`   | opens and closes the profiles sidebar (applies and re-scores)   |
| `L`        | activity log (collect/veto/notify/profile) — last 7 days        |
| `enter`    | opens the posting in the browser                                |
| `c`        | collects with a spinner and per-source progress, then re-scores |
| `n`        | notifies the top 5 through DMS                                  |
| `r`        | reloads from SQLite                                             |
| `q`        | quits                                                           |

The line between the body and the footer is the status bar: transient feedback
(collect, veto, reload) that disappears on its own after 3s. Vetoed postings do
not make the CLI's `top`/`notify` and are marked with `✕` when visible.

Filter: space-separated tokens, all of which must match (AND). `remote`/`onsite`
(work mode), `score:70` (minimum), `src:remotive` (source), `tipo:junior`
(level/contract), and any other word is a substring search over
title/company/tags. For example: `python remote score:60`.

## Sources: API vs scraping

Every source is upfront about its kind; `tabelavagas sources` shows it:

| Source       | kind       | how it is obtained                                                 |
| ------------ | ---------- | ------------------------------------------------------------------ |
| remotive     | `api`      | documented public JSON (remotive.com/api), no auth                 |
| programathor | `scraping` | server-rendered HTML parsed with goquery                           |
| greenhouse   | `api`      | boards-api.greenhouse.io (public JSON, no auth) — per company      |

- **api**: structured data from an official endpoint. A break means the vendor
  changed the contract.
- **scraping**: parsing server-rendered HTML — it can break when the site changes.
  That is why they are _best-effort_: a bad source becomes a warning on stderr and
  does not bring the collection down.

### Per-company sources (Greenhouse)

`greenhouse` is per company: create `~/.config/tabelavagas/sources.toml` with the
companies that use Greenhouse (public, no auth):

```toml
[greenhouse]
companies = ["gitlab", "vercel", "stripe", "airtable"]
```

Without the file (or without the section) the source does not run. Dedup is keyed
on `greenhouse:<company>-<id>`, so different companies never collide.

## Installing

```bash
go install github.com/TabelaDev/tabelavagas@latest
```

The DB goes to `~/.local/state/tabelavagas/vagas.db` (SQLite, pure Go driver).

## Profiles

Each profile defines keywords, bonuses and a cutoff to decide whether a posting is
"worth it". Built-in profiles:

| Profile     | MinScore | Focus keywords                                         |
| ----------- | -------- | ------------------------------------------------------ |
| `dev`       | 70       | python, svelte, typescript, go, backend, fullstack, ml |
| `data`      | 65       | dados, data science, ml, ia, python, matemática        |
| `fullstack` | 60       | fullstack, frontend, react, next, vue, node, js        |

### Custom profiles

Create `~/.config/tabelavagas/profiles.toml`:

```toml
[profiles.meuperfil]
min_score = 55
keywords = ["python", "ml", "ia", "dados"]
remote_bonus = 10
intern_bonus = 15
city_bonus = 8
city = ["belo horizonte", "bh"]
```

Custom profiles override built-ins of the same name.

Precedence: the `--profile` flag > the `TABELAVAGAS_PROFILE` env var > the config
file > the default (`dev`).

## Heuristic score

A base of 20 + hits on the profile's keywords (capped at 8 × 6 = 48 points) +
bonuses for remote, internship, junior and city. The cutoff varies per profile.

### BYOK LLM

Use `--scorer llm` to score with an LLM (OpenAI-compatible, DeepSeek by default).
The API is only called for new postings (cached by a hash of the content).

The key lives **only** in `TABELAVAGAS_LLM_API_KEY` — a secret does not go into a
config file. Provider, model and limits live under `[llm]` in `config.toml` (see
[Configuration](#configuration)), with `TABELAVAGAS_LLM_BASEURL` and
`TABELAVAGAS_LLM_MODEL` still winning over the file.

## Configuration

Optional, at `~/.config/tabelavagas/config.toml`. Without the file the app runs on
the defaults below; with it, only the keys present are overridden. `f5` reloads
without restarting.

```toml
default_profile = "dev"

[database]
path = "~/.local/state/tabelavagas/vagas.db"

[llm]
base_url    = "https://api.deepseek.com/v1"
model       = "deepseek-chat"
temperature = 0.1
max_tokens  = 100
workers     = 6      # size of the scoring pool
http_timeout = "30s"

[collector]
http_timeout           = "30s"
greenhouse_delay       = "500ms"
programathor_max_pages = 20
programathor_delay     = "400ms"

[layout]
sidebar_width = 22
card_height   = 4
detail_min    = 32
detail_max    = 64

[notify]
binary     = "dms"
timeout_ms = 8000
app_name   = "tabelavagas"
count      = 5       # how many postings the notification carries without an explicit N
```

`profiles.toml` and `sources.toml` remain files of their own — they are catalogues
(what a profile scores, which companies to scrape), not preferences. `config.toml`
only names the default profile.

Not everything is hot-reloadable: `[database].path` is read at boot only, and
`[llm].workers` takes effect from the next scoring run.

### Env

Env wins over the file, which wins over the default. The API key is env-only — a
secret does not go into a config file.

| Var                          | Overrides                                            |
| ---------------------------- | ---------------------------------------------------- |
| `TABELAVAGAS_LLM_API_KEY`    | — (env-only, required for `--scorer llm`)            |
| `TABELAVAGAS_DB`             | `[database].path`                                    |
| `TABELAVAGAS_PROFILE`        | `default_profile` (and `--profile` beats both)       |
| `TABELAVAGAS_LLM_BASEURL`    | `[llm].base_url`                                     |
| `TABELAVAGAS_LLM_MODEL`      | `[llm].model`                                        |

## Daily timer

The `install-timer.sh` script installs a systemd user timer that runs
`tabelavagas all --only-new` every day at 21:00 (heuristic, no LLM cost).
`--only-new` means only postings not notified yet become a notification — the ones
already sent are marked in the DB and are not re-notified. `Persistent=true`
guarantees a catch-up if the machine was off.

```bash
./install-timer.sh
```

## Roadmap

- [x] Interactive Bubble Tea TUI
- [x] `rank --min N` and profile through flags
- [x] Server-rendered HTML scraping for Programathor
- [x] Notify through DMS (desktop notification)
- [x] Named profiles (built-in + custom through TOML)
- [x] systemd timer at 21:00 (Persistent=true)
- [x] BYOK LLM (OpenAI-compatible, DeepSeek by default)

MIT © TabelaDev.
