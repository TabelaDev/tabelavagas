<div align="center">

# tabelhavagas

[English](README.md) · **Português**

[![Go Version](https://img.shields.io/github/go-mod/go-version/TAbelhaDev/tabelhavagas?style=flat-square&logo=go&logoColor=white&color=00ADD8)](go.mod)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue?style=flat-square)](LICENSE)
[![Built with Bubble Tea](https://img.shields.io/badge/built%20with-Bubble%20Tea-ff69b4?style=flat-square)](https://github.com/charmbracelet/bubbletea)
[![Powered by tabelhatuiui](https://img.shields.io/badge/theme-tabelhatuiui-d6b4f7?style=flat-square)](https://github.com/TAbelhaDev/tabelhatuiui)

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/ianptkcs)

</div>

---

Filtra vagas de programação que podem valer a pena para o teu perfil e avisa
só as que importam. Um monobando em Go: collect → rank → top → notify.

## CLI

```
tavagas                  roda TUI (padrão)
tavagas sources          lista fontes e o tipo (API vs scraping)
tavagas collect          baixa vagas das fontes e salva no SQLite
tavagas rank [flags]     score 0-100 das vagas salvas
tavagas top [N] [flags]  imprime as N melhores (default 10)
tavagas notify [N]       envia top N via desktop notification (DMS)
tavagas all [flags]      collect → rank → top → notify
```

Flags comuns:

| Flag          | Descrição                                       | Default |
| ------------- | ----------------------------------------------- | ------- |
| `--min N`     | score mínimo pra entrar no `top`/`notify`       | —       |
| `--profile X` | perfil de scoring (`dev`, `data`, `fullstack`)  | `dev`   |
| `--scorer X`  | `heuristic` ou `llm`                            | —       |
| `--only-new`  | só vagas ainda não notificadas (`top`/`notify`/`all`) | —    |
| `--dry`       | mostra o que faria sem gravar                   | —       |

## TUI

`tavagas` abre a TUI: lista de vagas novas (ainda não notificadas), cada
uma num card de 3 linhas (score+título, empresa/local/tipo, tags+deadline).
A vaga focada fica com destaque full-width na cor de acento.

| Tecla      | Ação                                        |
| ---------- | ------------------------------------------- |
| `j`/`k`    | navega (também setas, `g`/`G`, `pgup`/`pgdn`) |
| `h`/`l`    | troca de coluna na visão de faixas (`t`)    |
| `/`        | abre filtro live — digita e a lista reduz   |
| `esc`      | sai do filtro (`ctrl+u` limpa)              |
| `o`        | abre/fecha painel de detalhes               |
| `x`        | vetar/desvetar a vaga focada (some da lista) |
| `V`        | mostrar/esconder as vagas vetadas           |
| `t`        | visão de faixas: colunas 80-100 · 60-79 · <60 |
| `ctrl+e`   | abre/fecha sidebar de perfis (aplica e re-scoreia) |
| `L`        | log de atividade (collect/veto/notify/perfil) — últimos 7 dias |
| `enter`    | abre a vaga no navegador                    |
| `c`        | coleta com spinner + progresso por fonte e re-scoreia |
| `n`        | notifica as top 5 via DMS                    |
| `r`        | recarrega do SQLite                         |
| `q`        | sai                                        |

A linha entre o corpo e o footer é a barra de status: feedback transitório
(collect, veto, reload) que some sozinho depois de 3s. Vagas vetadas não
entram no `top`/`notify` do CLI e ficam marcadas com `✕` quando visíveis.

Filtro: tokens separados por espaço, todos combinam (AND). `remote`/`onsite`
(modo de trabalho), `score:70` (mínimo), `src:remotive` (fonte), `tipo:junior`
(nível/contrato), e qualquer palavra é busca por substring no título/empresa/
tags. Ex: `python remote score:60`.

## Fontes: API vs scraping

Cada fonte é transparente quanto ao tipo; `tavagas sources` mostra:

| Fonte        | tipo       | como é obtida                                                     |
| ------------ | ---------- | ----------------------------------------------------------------- |
| remotive     | `api`      | JSON público documentado (remotive.com/api), sem auth             |
| programathor | `scraping` | HTML server-rendered parseado via goquery                         |
| greenhouse   | `api`      | boards-api.greenhouse.io (JSON público, sem auth) — por empresa   |

- **api**: dado estruturado de endpoint oficial. Quebra = fornecedor mudou o contrato.
- **scraping**: parse de HTML/server-rendered — pode quebrar quando o site muda.
  Por isso são *best-effort*: fonte ruim vira aviso no stderr e não derruba a coleta.

### Fontes por empresa (Greenhouse)

`greenhouse` é por-empresa: crie `~/.config/tabelhavagas/sources.toml` com as
empresas que usam o Greenhouse (sem auth, públicas):

```toml
[greenhouse]
companies = ["gitlab", "vercel", "stripe", "airtable"]
```

Sem o arquivo (ou sem a seção), a fonte não roda. O dedup é por
`greenhouse:<empresa>-<id>`, então empresas diferentes não colidem.

## Como instalar

```bash
go install github.com/TAbelhaDev/tabelhavagas@latest
```

Isso instala o binário como `tabelhavagas` (nome do módulo). Pra ter o nome curto `tavagas`
usado no resto deste README, compile a partir do source:
`git clone https://github.com/TAbelhaDev/tabelhavagas.git && cd tabelhavagas && go build -o tavagas .`

DB vai para `~/.local/state/tabelhavagas/vagas.db` (SQLite, driver puro Go).

## Perfis

Cada perfil define keywords, bônus e cutoff pra decidir se uma vaga "vale a pena".
Perfis built-in:

| Perfil      | MinScore | Keywords foco                                          |
| ----------- | -------- | ------------------------------------------------------ |
| `dev`       | 70       | python, svelte, typescript, go, backend, fullstack, ml |
| `data`      | 65       | dados, data science, ml, ia, python, matemática        |
| `fullstack` | 60       | fullstack, frontend, react, next, vue, node, js        |

### Perfis customizados

Crie `~/.config/tabelhavagas/profiles.toml`:

```toml
[profiles.meuperfil]
min_score = 55
keywords = ["python", "ml", "ia", "dados"]
remote_bonus = 10
intern_bonus = 15
city_bonus = 8
city = ["belo horizonte", "bh"]
```

Perfis custom sobrescrevem built-ins com o mesmo nome.

Precedência: flag `--profile` > env `TABELHAVAGAS_PROFILE` > config file > default (`dev`).

## Score heurístico

Base 20 + hits de keywords do perfil (capped em 8 × 6 = 48 pontos) +
bônus remoto, estágio, júnior e cidade. Cutoff varia por perfil.

### BYOK LLM

Use `--scorer llm` pra pontuar com LLM (OpenAI-compatível, default DeepSeek).
Só chama a API pra vagas novas (cache por hash do conteúdo).

A chave fica **só** em `TABELHAVAGAS_LLM_API_KEY` — segredo não vai pra arquivo
de config. Provider, modelo e limites ficam em `[llm]` no `config.toml`
(veja [Configuração](#configuração)), com `TABELHAVAGAS_LLM_BASEURL` e
`TABELHAVAGAS_LLM_MODEL` ainda vencendo o arquivo.

## Configuração

Opcional, em `~/.config/tabelhavagas/config.toml`. Sem o arquivo o app roda nos
defaults abaixo; com ele, só as chaves presentes são sobrescritas. `f5`
recarrega sem reiniciar.

```toml
default_profile = "dev"

[database]
path = "~/.local/state/tabelhavagas/vagas.db"

[llm]
base_url    = "https://api.deepseek.com/v1"
model       = "deepseek-chat"
temperature = 0.1
max_tokens  = 100
workers     = 6      # tamanho do pool de scoring
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
app_name   = "tabelhavagas"
count      = 5       # quantas vagas a notificação leva sem N explícito
```

`profiles.toml` e `sources.toml` continuam arquivos próprios — são catálogos
(o que um perfil pontua, quais empresas raspar), não preferências. O
`config.toml` só aponta o perfil padrão pelo nome.

Nem tudo é recarregável a quente: `[database].path` é lido só no boot, e
`[llm].workers` vale a partir da próxima rodada de scoring.

### Env

Env vence o arquivo, que vence o default. A chave da API é env-only — segredo
não entra em arquivo de config.

| Var                          | Sobrescreve                                         |
| ---------------------------- | --------------------------------------------------- |
| `TABELHAVAGAS_LLM_API_KEY`    | — (env-only, obrigatória pro `--scorer llm`)        |
| `TABELHAVAGAS_DB`             | `[database].path`                                   |
| `TABELHAVAGAS_PROFILE`        | `default_profile` (e `--profile` vence os dois)     |
| `TABELHAVAGAS_LLM_BASEURL`    | `[llm].base_url`                                    |
| `TABELHAVAGAS_LLM_MODEL`      | `[llm].model`                                       |

## Timer diário

O script `install-timer.sh` instala um systemd user timer que roda
`tavagas all --only-new` todo dia às 21:00 (com heurística, sem custo
LLM). `--only-new` faz só as vagas ainda não notificadas virarem aviso — as
já enviadas ficam marcadas no DB e não re-notificam.
`Persistent=true` garante catch-up se a máquina estiver desligada.

```bash
./install-timer.sh
```

## Roadmap

- [x] TUI Bubble Tea interativo
- [x] `rank --min N` e perfil via flags
- [x] Scraping HTML server-rendered pro Programathor
- [x] Notify via DMS (desktop notification)
- [x] Perfis nomeados (built-in + custom via TOML)
- [x] systemd timer 21:00 (Persistent=true)
- [x] BYOK LLM (OpenAI-compatível, DeepSeek default)

MIT © TAbelhaDev.
