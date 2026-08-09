# tabelavagas

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
| `--min N`     | cutoff de score (override do `min_score`, no scoring) | —  |
| `--profile X` | perfil de scoring (`dev`, `data`, `fullstack`)  | `dev`   |
| `--scorer X`  | `heuristic` ou `llm`                            | —       |
| `--only-new`  | só vagas ainda não notificadas (`top`/`notify`/`all`) | —    |
| `--dry`       | mostra o que faria sem gravar                   | —       |

## Fontes: API vs scraping

Cada fonte é transparente quanto ao tipo; `tabelavagas sources` mostra:

| Fonte        | tipo       | como é obtida                                                     |
| ------------ | ---------- | ----------------------------------------------------------------- |
| remotive     | `api`      | JSON público documentado (remotive.com/api), sem auth             |
| gupy         | `api`      | API oficial api.gupy.io (Bearer via `TABELAVAGAS_GUPY_TOKEN`)     |
| programathor | `scraping` | HTML server-rendered parseado via goquery                         |

- **api**: dado estruturado de endpoint oficial. Quebra = fornecedor mudou o contrato.
- **scraping**: parse de HTML/server-rendered — pode quebrar quando o site muda.
  Por isso são *best-effort*: fonte ruim vira aviso no stderr e não derruba a coleta.

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
| `TABELAVAGAS_GUPY_TOKEN`     | Bearer token da API da Gupy (`developers.gupy.io`)  |
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
