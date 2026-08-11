# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Adicionado

- `~/.config/tabelavagas/config.toml` (opcional): perfil padrão, path do
  banco, provider/modelo/limites do LLM, timeouts e delays dos coletores,
  layout da TUI e os parâmetros de notificação. Só as chaves presentes
  sobrescrevem. `profiles.toml` e `sources.toml` seguem separados — são
  catálogos, não preferências.
- Tecla `f5`: recarrega config.toml e keybindings sem reiniciar.

Env continua vencendo o arquivo (`TABELAVAGAS_DB`, `_PROFILE`,
`_LLM_BASEURL`, `_LLM_MODEL`), resolvida no ponto de uso.
`TABELAVAGAS_LLM_API_KEY` é env-only: segredo não ganha chave de config.

### Corrigido

- A sidebar de perfis despachava teclas por literal cru, ignorando o
  KeyRegistry — quem remapeava uma tecla era atendido em todo lugar menos ali.
- `notify` tinha três "5" independentes pro mesmo conceito (`notifyCount` e
  duas vezes dentro do `runNotify`); todos leem `[notify].count` agora.
- README documentava `TABELAVAGAS_SCORER=llm`, que nenhum código jamais leu —
  o seletor sempre foi só a flag `--scorer`.

- **Score heurístico:** o casamento de keyword era `strings.Contains`, sem
  fronteira de palavra, e o perfil `dev` tem entradas curtas — `"ia"` casava com
  *experiência*, *tecnologia*, *ciência*, *dia*; `"ml"` com *html*; `"go"` com
  *algoritmo*, *jogo*, *categoria*. Como os acertos saturam em 8 (+48 pontos),
  quase toda descrição longa em PT-BR chegava perto do teto e o ranking virava
  função do tamanho do texto. Agora o casamento é por token, sobre texto
  normalizado sem acento.
- `"júnior"` estava duplicado no perfil `dev` e contava dois acertos pro mesmo
  termo; keyword repetida passou a contar uma vez só.
- A TUI não é mais corrompida por escrita em stdout/stderr. O scorer LLM emite
  um aviso por vaga que falha — com chave inválida, é uma linha por vaga em cima
  do frame do Bubble Tea. `collect`, `notify` e o scoring capturam a saída e
  mandam pro log de atividade.
- Cache do LLM: `jobHash` não incluía `Description` nem `Salary`, mas o prompt
  usa a descrição. Depois que `setDetails` preenchia a descrição, o score em
  cache — calculado sem ela — continuava valendo pra sempre.
- `save` virou upsert: anúncio que mudou depois da primeira coleta (salário
  publicado, deadline movido, título corrigido) nunca era atualizado. Veto,
  marca de notificado e os scores continuam intocados.
- Coletor: `User-Agent` próprio (o default do Go é bloqueado por boa parte dos
  sites), pausa entre páginas, e erro no meio da paginação deixou de sumir em
  silêncio — antes um resultado parcial era indistinguível de coleta completa.
- `openStoreAt` checa o erro de `MkdirAll` e fecha o handle quando a migração
  falha.

### Alterado

- Scoring por LLM roda num pool de 6 workers. Cada chamada é um POST com timeout
  de 30s; em série, algumas centenas de vagas tornavam o `rank --scorer llm`
  inviável. A conexão do SQLite ficou limitada a 1 pra evitar `SQLITE_BUSY`.

### Added
- Log de atividade persistente (tabela `activity`): collect, notify, veto,
  open, llm, profile — mantém os últimos 7 dias (prune automático) e a visão
  `L` carrega do DB (sobrevive a reinício). Badges coloridos por tipo.
- Fonte `greenhouse` (boards-api.greenhouse.io, API pública sem auth) com
  lista de empresas configurável em `~/.config/tabelavagas/sources.toml`
- Removida a fonte `gupy` (token inviável sem plano Premium/Enterprise)

### Fixed
- TUI não carregava vagas em DBs com colunas `salary`/`description` NULL
  (adicionadas via ALTER TABLE): `scanJob` agora aceita NULL nessas colunas
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
