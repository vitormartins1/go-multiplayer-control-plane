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
- **Revisão de produto 2026-09**: `main` voltou a ficar verde, o `HOST_KILLED` não se perde mais no reenvio
  (`session/029`) e o publish no SNS tem timeout próprio, `/admin` do session e do analytics sob a internal key,
  restart policy e chave persistente do identity no compose, teto de corpo nas rotas de jogador
  (`player-data/003`), idempotência do leaderboard com TTL (`leaderboard/008`), cobertura ≥ 92% no analytics e no
  achievements, primeiros testes do admin-ui, **demo reproduzível** (`scripts/demo-host-world.sh`) e docs de estado
  sincronizadas com o código. Ver `review/PROGRESSO.md` para os commits.

## Decisões do dono

D1–D6 da revisão foram **aprovadas em 2026-09-26** (e o aval para mexer em `shared/auth`) — executadas na Fase 3
(`review/PROGRESSO.md`). Abertas agora, na [estratégia de produto](review/ESTRATEGIA-PRODUTO-2026-09.md):

| Decisão | Recomendação |
|---|---|
| ~~Topologia P2P (rascunho B)~~ | **aprovada e feita no backend** (ADR-0032, `session/031`, contrato 1.4.0); falta o netcode no jogo (NGO + transporte Steam no modo host) |
| ~~Rascunho A virar ADR~~ | **feito** — ADR-0031 |
| Virar `SESSION_AUTH_MODE=enforce` | quando o jogo mandar bearer e chave de servidor |
| Abrir o repo como open source (Apache-2.0) | sim, depois do lançamento ou quando quiser a vitrine |

## Pendente de você (fora do código)

1. **Reinstalar o LaunchAgent do túnel Pi 5** (`infra/agones/pi5/pi5-tunnels.sh install`) — a ADR-0029 moveu o
   script; o LaunchAgent pode ainda apontar para o caminho antigo (pendência de 2026-07-11, não confirmada depois).
2. **Integração do cloud-save**: a VM da revisão não conseguiu baixar `minio/minio:latest` (`pull access denied`).
   Rodar `go test -tags=integration ./services/cloud-save-service/...` localmente; se falhar, a imagem saiu do
   Docker Hub e o compose e o teste precisam de outra origem.
3. **Recriar a stack local** depois de puxar esta branch: o compose ganhou restart policy, a `INTERNAL_API_KEY` do
   analytics e o secret do identity — rodar `infra/docker/gen-secrets.sh` e depois
   `docker compose -f infra/docker/docker-compose.yml up -d --build`; em seguida `scripts/demo-host-world.sh`
   (a VM da revisão não conseguiu buildar as imagens — validou com os binários no host).
4. **Validar com o jogo** (quando a fase multiplayer voltar): reenvio do `HOST_KILLED` com o fan-out fora
   (derrubar o localstack, matar o host, subir o localstack — o evento deve chegar ao achievements).

## Backlog priorizado

Executáveis sem decisão (próximas sessões):

1. **Espelhar conquistas na Steam** (consumidor do fan-out → `SetUserStatsForGame`).
2. **Unificar Rooms e Sessions** do jogo (tabela e detalhe compartilhados).
3. **Consolidar as séries diárias do analytics** numa tabela (pede ADR sobre a ADR-0021).
4. **ESLint no admin-ui** e **`govulncheck` no CI** (validar localmente antes).
5. **Refatorar `session_service.go`** (1,1k linhas) com a cobertura atual como rede.

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
