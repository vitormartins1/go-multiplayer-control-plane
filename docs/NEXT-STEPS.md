# NEXT-STEPS — handoff (atualizado 2026-06-30)

Ponto de retomada após limpar o contexto. Resume **o estado atual**, **o que fica pendente de você**, **o que foi feito nesta sessão** e o **backlog priorizado**.

## Estado atual (`main` verde, stack local de pé)

Gameplay loop fechado e observável no painel:

`FindMatch(game_id) → match por jogo → CreateSession(game_id) → ReportResult → fan-out (achievements + leaderboard) → painel`

- **Multi-tenant por jogo** (ADR-0017) ponta a ponta: game-registry + achievements + leaderboard + player-data + cloud-save escopados por `game_id`.
- **Auth serviço-a-serviço** (ADR-0018): internal key (`X-Internal-Key`) nos control planes; `shared/auth` (`RequireInternalKey` + `InternalKeyTransport`). Chave dev: `dev-internal-key`.
- **Presença por jogo** (ADR-0019): identidade continua **global**; cada jogo tem uma player base explícita (`game_players` no game-registry), marcada no **login-into-game** (identity aceita `gameId` opcional, marca best-effort com timeout de 2s).
- **Fan-out** do resultado: `session` → `CompositePublisher` → achievements + leaderboard. Best-effort.
- **Painel jogo-cêntrico** (admin-bff + admin-ui, Mantine) em http://localhost:8089 — ver seção abaixo.

10 serviços Go construídos: identity, game-registry, achievements, player-data, cloud-save, leaderboard, matchmaking (gRPC), session (gRPC), admin-bff, admin-ui. Observabilidade: Prometheus + Grafana + Tempo.

ADRs recentes: 0016 (painel BFF), 0017 (multi-tenant), 0018 (service auth), **0019 (presença por jogo)**.

## ⚠️ Pendente de você (ações fora do código)

1. **Push pro GitHub** — o repo ainda é **local**. Criar `github.com/vitormartins1/go-game-backend`, `git remote add origin …`, `git push -u origin main`. **Só então o CI (GitHub Actions) roda de verdade.** Reproduzi os 4 jobs verdes localmente, mas o ambiente do Actions nunca rodou (testcontainers às vezes pede ajuste no runner).
2. **Nomes de repo decididos:** `go-game-backend` (este) e `go-game-unity` (o data plane Unity, repo separado — ADR-0010). O module path já foi renomeado de `c130n/game-backend` → **`github.com/vitormartins1/go-game-backend`**.

## Painel jogo-cêntrico (✅ completo)

Modelo final = **dois eixos** (identidade global + presença; ver ADR-0019). Não há mais "jogo ativo global" no header.

- **Games** (`admin-ui/005-game-library` + `003-generated-cover`): biblioteca de **capas 9:16** geradas no cliente por **DiceBear** a partir de `generatedImageSeed` (=slug); metadados no game-registry (`image`/`imageSource`/`generatedImageSeed`/`generatedImageStyle`) deixam espaço p/ upload futuro. Abrir um jogo → abas **Achievements** (CRUD completo, `achievements/006-catalog-crud`), **Leaderboards**, **Players** (base do jogo = presença), Configurações (ComingSoon).
- **Players**: base **global** do identity (paginada) + **avatares DiceBear** `adventurer` (seed = playerId, `admin-ui/006-player-avatars`). Abrir um player → **seletor escopado aos jogos DELE** (`GET /api/players/{id}/games`, consulta reversa da presença, `game-registry/004-games-of-player`) → abas Achievements/Player data/Cloud saves.

## O que foi feito nesta sessão (para referência)

Todas mergeadas na `main` (branch por feature, `--no-ff`):

- `player-data/002-game-scope`, `cloud-save/002-game-scope`, `admin-bff/006-player-tabs` — game-scope + admin read + abas Player data/Cloud saves.
- `admin-ui/005-game-library` — biblioteca de capas + (depois removido) jogo ativo global.
- `game-registry/002-game-presence` (**ADR-0019**) — presença + login-into-game + Players por jogo.
- `game-registry/003-generated-cover` — capa determinística por seed (DiceBear no cliente).
- `admin-ui/006-player-avatars` — avatar humanoide por jogador.
- `game-registry/004-games-of-player` — **review do modelo**: navegação player-cêntrica, remove o jogo ativo global, base por jogo no GameDetail, consulta reversa.
- `achievements/006-catalog-crud` — editar/remover conquistas (delete transacional: apaga progresso + catálogo).
- **chore**: module path renomeado (regenerar proto com `buf` — um `sed` cru corrompe o `rawDesc`); **CI GitHub Actions**; **code-splitting** do admin-ui (lazy routes + `manualChunks`: bundle 941kB → entry 28kB, DiceBear/xyflow saem do boot).
- **code-review** (`high`, `fd25bf4..HEAD`): 4 finders em paralelo. Corrigidos: 4 bugs de dados obsoletos no painel (deps `playerId` + reset do jogo ao trocar de jogador), PUT de conquista devolve `eventType`, loginAnonymous tolera corpo vazio/chunked, `markPresence` com timeout 2s. Rastreio cross-file e convenções vieram limpos.

## Backlog priorizado (sem spec ainda)

Uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (`docs/GIT-WORKFLOW.md`). Ordem sugerida:

1. **Push pro GitHub** (acima) → CI passa a rodar.
2. **Fan-out resiliente** — hoje `ReportResult → achievements/leaderboard` e a presença são best-effort síncronos (logam falha). Evoluir p/ **outbox/retry** (ou fila) para garantir entrega. Provável ADR (SQS/outbox table). *É o ponto mais frágil do sistema.*
3. **analytics-service** (Fase 9 do ROADMAP — **único serviço não construído**; ≈ Game Analytics Pipeline). Ingestão de eventos + agregação; acende a aba **Analytics** do painel (hoje "em breve"). SDD do zero.
4. **Editar/remover jogo** no painel — *rename* é simples (cabe na aba Configurações do GameDetail, hoje ComingSoon). *Delete* precisa de decisão: dado órfão por `game_id` em achievements/player-data/cloud-save/leaderboard/presença (cascata cross-service, provável ADR).
5. **Leaderboard**: seasons/janelas por período + paginação.
6. **Cleanup admin-bff** (do code-review, não é bug): `do()`/`classify()`/`errorCode()`/default-client duplicados nos 4 adapters + o presence client do identity → extrair um helper `internal/upstream` parametrizado pelo erro sentinela.
7. **Service JWT** (evolução do ADR-0018): trocar a internal key compartilhada por tokens por serviço (identidade, expiração, revogação). Quando houver múltiplos ambientes/consumidores. Abrir ADR.
8. **Infra / deploy — o fim, e o payoff da tese (ADR-0008):** não existe `infra/terraform/` ainda (tudo é Docker Compose, local-first deliberado). Terraform → LocalStack → AWS; compute por serviço (ADR-0005: Lambda/Fargate/ECS/EC2/EKS); medir custo/performance vs AWS GameKit.
9. **Data plane / o jogo (repo `go-game-unity`, não-Go):** Unity Dedicated Server + Netcode + Agones/EKS (ADR-0010/0011/0012). Onde o gameplay realtime roda.

## Gotchas operacionais

- **Migrations Postgres** só rodam em **volume novo**: ao mudar/adicionar migration, `docker compose ... rm -sfv <svc> <svc>-postgres` (apaga o volume anônimo) e remontar a migration no volume do compose. `game-registry-postgres` já teve 3 recriações (migrations 003/004).
- **Proto**: após mexer no module path / `.proto`, **regenerar com `buf`** (`PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço) — um `sed` no `.pb.go` corrompe o `rawDesc` (go_package tem prefixo de tamanho).
- **golangci-lint** não está instalado por padrão: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` (o CI usa a mesma versão pinada). Config em `.golangci.yml` (v2): errcheck dispensado só nos `_test.go`.
- **Prometheus** não raspa job novo sem `docker compose restart prometheus`; **Grafana** provisioning só no boot (restart do container).
- **Processo órfão** segurando porta gRPC (50051/50052) de algum `go run` antigo: matar antes de `up`.
- **/metrics e /healthz** ficam **fora** do guard da internal key (Prometheus precisa raspar).
- Internal key dev: `dev-internal-key` (header `X-Internal-Key`).
- **admin-ui bundle**: DiceBear `adventurer` é pesado (~280kB); já mitigado com code-splitting (só carrega em Games/Players).
