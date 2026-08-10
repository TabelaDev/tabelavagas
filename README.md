<div align="center">

# tabelavagas

[![Go Version](https://img.shields.io/github/go-mod/go-version/TabelaDev/tabelavagas?style=flat-square&logo=go&logoColor=white&color=00ADD8)](go.mod)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue?style=flat-square)](LICENSE)
[![Built with Bubble Tea](https://img.shields.io/badge/built%20with-Bubble%20Tea-ff69b4?style=flat-square)](https://github.com/charmbracelet/bubbletea)
[![Powered by tabelatuiui](https://img.shields.io/badge/theme-tabelatuiui-d6b4f7?style=flat-square)](https://github.com/TabelaDev/tabelatuiui)

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/ianptkcs)

</div>

---

Filtra vagas de programação que podem valer a pena para o teu perfil e avisa
só as que importam. Um monobando em Go: collect → rank → top → notify.

## CLI

```
tabelavagas                  roda TUI (padrão)
tabelavagas sources          lista fontes e o tipo (API vs scraping)
tabelavagas collect          baixa vagas das fontes e salva no SQLite
tabelavagas rank [flags]     score 0-100 das vagas salvas
tabelavagas top [N] [flags]  imprime as N melhores (default 10)
tabelavagas notify [N]       envia top N via desktop notification (DMS)
tabelavagas all [flags]      collect → rank → top → notify
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

`tabelavagas` abre a TUI: lista de vagas novas (ainda não notificadas), cada
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

Cada fonte é transparente quanto ao tipo; `tabelavagas sources` mostra:

| Fonte        | tipo       | como é obtida                                                     |
| ------------ | ---------- | ----------------------------------------------------------------- |
| remotive     | `api`      | JSON público documentado (remotive.com/api), sem auth             |
| programathor | `scraping` | HTML server-rendered parseado via goquery                         |
| greenhouse   | `api`      | boards-api.greenhouse.io (JSON público, sem auth) — por empresa   |

- **api**: dado estruturado de endpoint oficial. Quebra = fornecedor mudou o contrato.
- **scraping**: parse de HTML/server-rendered — pode quebrar quando o site muda.
  Por isso são *best-effort*: fonte ruim vira aviso no stderr e não derruba a coleta.

### Fontes por empresa (Greenhouse)

`greenhouse` é por-empresa: crie `~/.config/tabelavagas/sources.toml` com as
empresas que usam o Greenhouse (sem auth, públicas):

```toml
[greenhouse]
companies = ["gitlab", "vercel", "stripe", "airtable"]
```

Sem o arquivo (ou sem a seção), a fonte não roda. O dedup é por
`greenhouse:<empresa>-<id>`, então empresas diferentes não colidem.

## Como instalar

```bash
go install github.com/TabelaDev/tabelavagas@latest
```

DB vai para `~/.local/state/tabelavagas/vagas.db` (SQLite, driver puro Go).

## Perfis

Cada perfil define keywords, bônus e cutoff pra decidir se uma vaga "vale a pena".
Perfis built-in:

| Perfil      | MinScore | Keywords foco                                          |
| ----------- | -------- | ------------------------------------------------------ |
| `dev`       | 70       | python, svelte, typescript, go, backend, fullstack, ml |
| `data`      | 65       | dados, data science, ml, ia, python, matemática        |
| `fullstack` | 60       | fullstack, frontend, react, next, vue, node, js        |

### Perfis customizados

Crie `~/.config/tabelavagas/profiles.toml`:

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

Precedência: flag `--profile` > env `TABELAVAGAS_PROFILE` > config file > default (`dev`).

## Score heurístico

Base 20 + hits de keywords do perfil (capped em 8 × 6 = 48 pontos) +
bônus remoto, estágio, júnior e cidade. Cutoff varia por perfil.

### BYOK LLM

Use `--scorer llm` ou `TABELAVAGAS_SCORER=llm` pra pontuar com LLM
(OpenAI-compatível, default DeepSeek). Só chama a API pra vagas novas
(.cache por hash do conteúdo).

Env LLM:

| Var                        | Padrão                          |
| -------------------------- | ------------------------------- |
| `TABELAVAGAS_LLM_API_KEY`  | — (obrigatório pro LLM)        |
| `TABELAVAGAS_LLM_BASEURL`  | `https://api.deepseek.com/v1`  |
| `TABELAVAGAS_LLM_MODEL`    | `deepseek-chat`                 |

## Env

| Var                          | Padrão                                              |
| ---------------------------- | --------------------------------------------------- |
| `TABELAVAGAS_DB`             | `~/.local/state/tabelavagas/vagas.db`               |
| `TABELAVAGAS_PROFILE`        | perfil padrão (override de `--profile`)             |
| `TABELAVAGAS_LLM_API_KEY`    | chave API OpenAI-compatível (DeepSeek etc.)         |
| `TABELAVAGAS_LLM_BASEURL`    | base URL do provider LLM                             |
| `TABELAVAGAS_LLM_MODEL`      | modelo LLM                                           |

## Timer diário

O script `install-timer.sh` instala um systemd user timer que roda
`tabelavagas all --only-new` todo dia às 21:00 (com heurística, sem custo
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

MIT © TabelaDev.
