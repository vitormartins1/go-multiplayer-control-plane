# NEXT-STEPS — handoff (atualizado 2026-09-26)

Ponto de retomada. Diz **onde o projeto está**, **o que depende do dono** e **o que fazer a seguir**. O histórico
detalhado de cada fatia vive no `git log` (Conventional Commits com escopo) e nas `specs/` de cada serviço — este
doc não é changelog. A análise completa que embasa esta versão está em
[`review/REVISAO-2026-09.md`](review/REVISAO-2026-09.md); o andamento da execução, em
[`review/PROGRESSO.md`](review/PROGRESSO.md).

## Onde estamos

- **Control plane completo para os 4 arquétipos** (ADR-0023/0024). `host_world_dropin` é usado de verdade pelo
  **Folclore Souls** (ex-Capoeira Souls; Fases 0–5 do data plane, e2e k3d→Pi 5 em 2026-07-11); `queue_arena` pelo
  protótipo Packet Arena; `battle_royale`/`coop_mission` têm contrato e código, **sem jogo real**.
- **O jogo está em fase de gameplay** desde agosto (bosses, combate, mundo) — os pedidos de multiplayer estão
  adiados do lado de lá (`review/SINAIS-DO-FOLCLORE-SOULS.md`). O backend se prepara onde o desenho está claro, sem
  correr na frente.
- **Desde o último handoff (2026-07-13)** entraram: analytics por modo em todos os rollups e session health
  (analytics 012–015, session 027/028, admin-bff 025–028), ~27 specs de polimento do admin-ui (063–089), o fix
  "participação não fica refém do publish" (`49e976d`, 2026-08-19) e o endpoint de slug para o rename do Folclore
  Souls (`7f174a4`, spec retroativa `game-registry/014`).
- **Revisão de produto 2026-09** (esta sessão): `main` voltou a ficar verde, o `HOST_KILLED` não se perde mais no
  reenvio (`session/029`), `/admin` do session e do analytics sob a internal key, restart policy no compose, docs
  de estado sincronizadas com o código. Ver `review/PROGRESSO.md` para os commits.

## Decisões pendentes do dono (detalhe e recomendação em `review/REVISAO-2026-09.md` §6)

| # | Decisão | Recomendação |
|---|---|---|
| D1 | Autenticar o gRPC do session (bearer do jogador no `OpenWorld`/`JoinWorld`; credencial do game server nos reports) — muda o contrato | sim, em duas fatias com período de compatibilidade; ADR evoluindo a 0018 |
| D2 | Invasão filtrar por modo (`mode_id` opcional no `ListInvadableWorlds`) | sim |
| D3 | Boss no `BOSS_DEFEATED`: convenção em `stats` × campo novo | convenção em `stats` agora |
| D4 | Remover os dados fabricados do console (Fleet/Infrastructure) | remover |
| D5 | `battle_royale`/`coop_mission` | congelar e rotular "experimental" |
| D6 | Auth do console antes de qualquer deploy | decidir junto com o deploy |

## Pendente de você (fora do código)

1. **Reinstalar o LaunchAgent do túnel Pi 5** (`infra/agones/pi5/pi5-tunnels.sh install`) — a ADR-0029 moveu o
   script; o LaunchAgent pode ainda apontar para o caminho antigo (pendência de 2026-07-11, não confirmada depois).
2. **Integração do cloud-save**: a VM da revisão não conseguiu baixar `minio/minio:latest` (`pull access denied`).
   Rodar `go test -tags=integration ./services/cloud-save-service/...` localmente; se falhar, a imagem saiu do
   Docker Hub e o compose e o teste precisam de outra origem.
3. **Recriar a stack local** depois de puxar esta branch: o compose ganhou restart policy e a `INTERNAL_API_KEY`
   do analytics (`docker compose -f infra/docker/docker-compose.yml up -d --build`).
4. **Validar com o jogo** (quando a fase multiplayer voltar): reenvio do `HOST_KILLED` com o fan-out fora
   (derrubar o localstack, matar o host, subir o localstack — o evento deve chegar ao achievements).

## Backlog priorizado

Executáveis sem decisão (próximas sessões), por valor/esforço — detalhe e critério de "feito" em
`review/REVISAO-2026-09.md` §5:

1. **Demo reproduzível** do fluxo host_world (`login → OpenWorld → Invade → ReportParticipationEvent → conquista`)
   num script com o compose mínimo — a primeira coisa que alguém de fora vai querer ver.
2. **Testes no admin-ui**: Vitest nos `lib/` puros (`rulesGraph.ts`, `csv.ts`, `matchCond`) + ESLint no CI.
3. **Endurecimento HTTP** sem tocar `shared/`: `ReadHeaderTimeout`/`IdleTimeout`, `MaxBytesReader`.
4. **Chave de assinatura persistente do identity** no compose (hoje efêmera: tokens morrem a cada restart).
5. **TTL no set de idempotência do leaderboard** e **`govulncheck` no CI**.
6. **Refatorar `session_service.go`** (1,1k linhas, 4 arquétipos) só com a cobertura atual (95%) garantindo.

Dependem de decisão/evidência (não começar sem):

- Tudo de D1–D6 acima.
- Alocação sob demanda, reconexão, pools por região, fila de invasão, persistência de mundos abertos — quando o
  Folclore Souls voltar à fase multiplayer (`game-models/host_world_dropin-follow-ups.md`).
- Deploy AWS + medição de custo da tese (ADR-0008); Service JWT (evolução da ADR-0018); V2c do motor de regras;
  janela de tempo/streak no motor de conquistas; Fatia 2 da ADR-0029.

## Gotchas operacionais

- **Migrations Postgres** só rodam em **volume novo**: ao mudar/adicionar migration, ou aplicar manualmente no volume em uso (`docker exec -i <postgres> psql -U dev -d <db> < migration.sql`) ou `docker compose ... rm -sfv <svc> <svc>-postgres` (apaga o volume anônimo) e remontar. Já mordeu várias vezes nesta sessão (achievements/010, analytics/006 — mount que faltava desde a spec 007 original).
- **Proto**: após mexer no module path / `.proto`, **regenerar com `buf`** (`PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço) — um `sed` no `.pb.go` corrompe o `rawDesc` (go_package tem prefixo de tamanho).
- **golangci-lint** não está instalado por padrão: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` (o CI usa a mesma versão pinada). Config em `.golangci.yml` (v2): errcheck dispensado só nos `_test.go`.
- **Prometheus** não raspa job novo sem `docker compose restart prometheus`; **Grafana** provisioning só no boot (restart do container).
- **Fan-out (LocalStack/Terraform):** os recursos SNS/SQS são criados pelo `messaging-provisioner` (Terraform) no boot — depende do LocalStack `healthy`. As envs `MATCH_RESULTS_TOPIC_ARN`/`MATCH_RESULTS_QUEUE_URL` no compose são **fixas e determinísticas**. Se mudar o `.tf`, o provisioner reaplica no próximo `up` (idempotente).
- **Processo órfão** segurando porta gRPC (50051/50052) de algum `go run` antigo: matar antes de `up`.
- **/metrics e /healthz** ficam **fora** do guard da internal key (Prometheus precisa raspar).
- Internal key dev: `dev-internal-key` (header `X-Internal-Key`).
- **admin-ui bundle**: DiceBear `adventurer` é pesado (~280kB); já mitigado com code-splitting. `@mantine/charts`/`recharts` (LineChart) também entram em chunk lazy próprio.
- **Canvas do editor de regras** (`rulesGraph.ts`): alturas de nó são ESTIMADAS no layout — se um card mudar de conteúdo/CSS, recalibrar medindo `offsetHeight` real via Playwright (gotcha batido várias vezes nas specs 049/050/052).
