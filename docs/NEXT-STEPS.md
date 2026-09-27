# NEXT-STEPS — handoff (atualizado 2026-09-27)

Ponto de retomada. Diz **onde o projeto está**, **o que depende do dono** e **o que fazer a seguir**. O histórico
detalhado de cada fatia vive no `git log` (Conventional Commits com escopo) e nas `specs/` de cada serviço — este
doc não é changelog.

## Onde estamos

- **Control plane completo para os 4 arquétipos** (ADR-0023/0024). `host_world_dropin` é usado de verdade pelo
  **Folclore Souls** (ex-Capoeira Souls; Fases 0–5 do data plane, e2e k3d→Pi 5 em 2026-07-11); `queue_arena` pelo
  protótipo Packet Arena; `battle_royale`/`coop_mission` têm contrato e código, **sem jogo real**.
- **O jogo está em fase de gameplay** desde agosto (bosses, combate, mundo) — os pedidos de multiplayer estão
  adiados do lado de lá. O backend se prepara onde o desenho está claro, sem
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
  sincronizadas com o código.
- **Fase 3 e rodada final (2026-09-26/27)**: autenticação no gRPC do session, login Steam, conquistas espelhadas
  na Steam, **topologia P2P** e **heartbeat do mundo** (contrato 1.1.0 → 1.5.0); console com hospedagem e sinal
  de vida; code review da branch inteira com os achados corrigidos (reenvio do `ReportResult`, limite de tentativas
  no login, lease do espelho Steam, `session_service.go` dividido); CI com ESLint e `govulncheck` bloqueante; o S3
  local trocado de MinIO para LocalStack (a imagem saiu do Docker Hub); README, arquitetura e produto reescritos,
  com [vídeo e imagens do produto](media/README.md). Tudo no [PR #1](https://github.com/vitormartins1/go-multiplayer-control-plane/pull/1).

## Decisões do dono

D1–D6 da revisão foram **aprovadas em 2026-09-26** (e o aval para mexer em `shared/auth`) — executadas na Fase 3. Abertas agora:

| Decisão | Recomendação |
|---|---|
| ~~Topologia P2P (rascunho B)~~ | **aprovada e feita no backend** (ADR-0032, `session/031`, contrato 1.4.0); falta o netcode no jogo (NGO + transporte Steam no modo host) |
| ~~Rascunho A virar ADR~~ | **feito** — ADR-0031 |
| Virar `SESSION_AUTH_MODE=enforce` | quando o jogo mandar bearer e chave de servidor |
| Abrir o repo como open source (Apache-2.0) | sim, depois do lançamento ou quando quiser a vitrine |

**O que depende de você** (chaves da Steam, Unity, `enforce`, decisões, validações locais), com o passo a passo:
[`PENDENCIAS-DO-DONO.md`](PENDENCIAS-DO-DONO.md).

**Integração do jogo:** o que o projeto Unity precisa fazer (credenciais, login Steam, heartbeat, P2P) está em
[`integration/UNITY-GUIA-INTEGRACAO.md`](integration/UNITY-GUIA-INTEGRACAO.md), com um prompt pronto para o agente
de lá.

## Pendente de você (fora do código)

1. **Reinstalar o LaunchAgent do túnel Pi 5** (`infra/agones/pi5/pi5-tunnels.sh install`) — a ADR-0029 moveu o
   script; o LaunchAgent pode ainda apontar para o caminho antigo (pendência de 2026-07-11, não confirmada depois).
2. ~~Integração do cloud-save~~ — **resolvido**: a imagem do MinIO saiu mesmo do Docker Hub; teste e compose usam o
   S3 do LocalStack.
3. **Recriar a stack local** depois de puxar esta branch: o compose ganhou restart policy, a `INTERNAL_API_KEY` do
   analytics e o secret do identity — rodar `infra/docker/gen-secrets.sh` e depois
   `docker compose -f infra/docker/docker-compose.yml up -d --build`; em seguida `scripts/demo-host-world.sh`
   (a VM da revisão não conseguiu buildar as imagens — validou com os binários no host).
4. **Validar com o jogo** (quando a fase multiplayer voltar): reenvio do `HOST_KILLED` com o fan-out fora
   (derrubar o localstack, matar o host, subir o localstack — o evento deve chegar ao achievements).

## Backlog priorizado

Executáveis sem decisão (próximas sessões):

Os cinco itens da lista anterior (espelho Steam, Rooms/Sessions unificados, ESLint, `govulncheck`, refatorar o
`session_service.go`) estão feitos; o da consolidação das séries diárias do analytics espera a ADR (o rascunho está
no histórico do git: `git show 6b8b0c7:docs/review/RASCUNHO-ADR-analytics-daily.md`). Candidatos para a próxima sessão:

1. **Consolidar as séries diárias do analytics** assim que o rascunho de ADR for aprovado.
2. **Limite de login distribuído** (Redis) quando houver mais de uma réplica do identity.
3. **Login no console (D6)**, junto com o primeiro deploy fora da máquina local.
4. ~~Login da Epic~~ (ADR-0033): **feito** (spec `identity/008`); falta o client ID do produto no EOS para ligar.

Backlog de engines e lojas (ADR-0033), por custo: espelho de conquistas na Epic (confirmar a API de servidor
confiável), SDK Unity empacotado (quando a camada C# do Folclore estabilizar), SDKs de Godot e Unreal (sob demanda),
cliente web em Three.js (pede ponte gRPC-web/Connect e transporte WebSocket/WebRTC), GOG (pesquisar as APIs de
servidor).

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
