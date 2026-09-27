# Progresso da revisão de produto (2026-09)

> Nota de continuidade da sessão autônoma (`docs/prompts/PROMPT-REVISAO-PRODUTO.md`). Revisão completa:
> [`REVISAO-2026-09.md`](REVISAO-2026-09.md). Atualizada a cada item.

**Branch:** `claude/revisao-produto-l2k7hr` (o prompt sugere `review/produto-2026-09`; a sessão na nuvem só
permite push para esta branch). Nada vai para a `main` — o dono revisa e mergeia (`--no-ff`, como de costume).
Commits diretos na branch, um por entrega; features com spec (`session/029`, `player-data/003`) no mesmo commit do código.

## Feito

| Item | Commit | O que mudou | Validação |
|---|---|---|---|
| Fase 1 | `34c1963` | `REVISAO-2026-09.md` | — |
| E1 | `335bd16` | `main` verde: teste do admin-bff sem `Directory`; 3 testes de integração do analytics com data fixa | `go test ./... -race` e integração verdes |
| E2 | `292c02a` | reenvio do `HOST_KILLED` republica (spec `session/029`) + contrato HWD corrigido | unit + **ao vivo** (LocalStack pausado) |
| E3 | `58e2ffe` | `/admin` do session e do analytics sob a internal key | ao vivo: 401 sem chave, 200 com chave |
| E4 | `9ece5ff` | `restart: unless-stopped` em 31 serviços do compose | `docker compose config` |
| E5 | `de4f901` | docs de estado sincronizadas (README, NEXT-STEPS, ARCHITECTURE, CLAUDE/AGENTS, OBSERVABILITY, game-models, specs velhas, spec retroativa `game-registry/014`) | links verificados; quickstart do identity rodado |
| E6 | `71dea5b` | 2 rotas mortas do admin-bff + cliente sem uso no admin-ui | `go test`, `tsc` |
| E7 | `69f4403` | cobertura: analytics service 60,9% → 92,8%; achievements 77,7% → 93,4% | `go test -cover` |
| P5 | `092b1eb` | chave persistente do identity no compose; `gen-secrets.sh` no quickstart (o compose falhava sem ele num clone limpo) | `kid` estável entre boots |
| P1 | `5a6ebbb` | `scripts/demo-host-world.sh` — fluxo do Folclore Souls ponta a ponta | **ao vivo**, passa de ponta a ponta |
| extra | `9b58304` | timeout de 5s no publish SNS (achado da bancada: report pendurava com o SNS travado) | ao vivo: `Internal` em ~5s |
| P4 | `2690675` | teto de corpo nas rotas de jogador; player-data 400 KiB → 413 (spec `player-data/003`) | unit |
| P3 | `5e2aaf2` | guarda de idempotência do leaderboard com TTL de 30 dias (spec `leaderboard/008`) + GO-LEARNINGS §38/§39 | integração com Redis real |
| P2 | `c57abe0` | Vitest no admin-ui (12 testes nos `lib/` puros) + `npm test` no CI | `npm test`, `npm run build` |
| V8 | `11a72c4` | game-registry: params do perfil com tipo errado → 400 (antes zero-value silencioso) | unit |
| verificação final | — | `go build`/`vet`/lint (0 issues)/unit (51 pacotes ok)/integração (57 ok; só o cloud-save falha por imagem MinIO inacessível na VM)/admin-ui; **demo ponta a ponta com todos os binários reconstruídos** | ✅ |

## Fase 3 — decisões aprovadas (2026-09-26)

O dono aprovou a revisão, seguiu todas as recomendações (D1–D6 + aval para `shared/auth`) e pediu para seguir com
autonomia no que não depende de gameplay, inclusive mudança de escopo.

| Item | Commit | O que mudou | Validação |
|---|---|---|---|
| D4 | `2d6624d` | console sem dados fabricados: Fleet 522 → 207 linhas (só localização/réplicas reais + cadastro); Infrastructure sem a aba de custos inventados, com atalho para o Grafana | tsc, vitest, build |
| D5 | `6d18f2b` | `battle_royale`/`coop_mission` rotulados experimentais (docs + seletor do console) | tsc |
| G3/G4 | `213112c` | `shared/auth`: `iss`/`aud`/`exp` obrigatórios; refetch de JWKS por `kid` desconhecido 1×/30s e coalescido (singleflight); `kid` revogado não cai na chave antiga | 6 testes novos, `-race` |
| D2/D3 | `2b555b8` | invasão respeita o modo (spec `matchmaking/012`); convenção de boss em `stats`; `docs/contracts/CHANGELOG.md` (1.0.0 → 1.1.0) | unit + transport |
| — | `d90512d` | CI: `buf breaking` dos protos contra a `main` | local: passa e detecta quebra |
| D1 | `ad89442` | autenticação no gRPC do session (spec `session/030`, ADR-0030), `warn` → `enforce`; `SummonSign` só pelo host | unit + **demo ao vivo em `enforce`** |
| Analytics | `4c5238b` | overview cross-game por **sessões** (todos os arquétipos); o Folclore aparecia zerado (spec `analytics/016`) | integração + BFF |
| Console | `8ba454d` | `lib/vocab.ts` (rótulos únicos; corrige `FAIL`), "Resumo do mundo" para host_world, pipeline vira tabela (−430 linhas), coluna Sessão nas salas (spec `admin-ui/090`) | Playwright com 5 mundos reais |
| Produto | `af3a677` | `ESTRATEGIA-PRODUTO-2026-09.md`: crítica do colega, Steam/GOG, SaaS × Asset Store × open source, SDK | — |
| Steam | `aea9f87` | login federado Steam no identity (spec `identity/005-steam-login`), contrato 1.3.0 | unit + integração + **ao vivo com Steam falsa** |
| Console | `65a2baa` | Rooms e Sessões do jogo na mesma `SessionTable` (spec `admin-ui/092`): ~1000 → ~800 linhas; a aba do jogo ganha slots por papel e o painel completo | vitest (32), build, Playwright |
| CI | ver `git log` | ESLint no admin-ui (hooks com dependência faltando = erro; 3 casos corrigidos) + job `govulncheck` report-only | `npm run lint` (0 erros), vitest, build |
| Console | `299fca3` | mapa de serviços com o analytics e as dependências reais; capturas novas na documentação | Playwright |
| ADRs | `aa344f1` | ADR-0031 (identidade federada, rascunho A) e spec `session/031` em rascunho | — |
| P2P | `43441db` | topologia p2p no `host_world_dropin` (spec `session/031`, ADR-0032, contrato 1.4.0): session sem GameServer + world token, registry, matchmaking, admin "P2P · host" | unit + **demo ao vivo p2p e dedicado em `enforce`** |
| Heartbeat | `2ffa569` | heartbeat do mundo (spec `session/032`, contrato 1.5.0): mundo que avisa não fecha mais aos 30 min; host caído fecha em ~90 s como `abandoned` | unit + **demo ao vivo nos dois modos** |
| Guia Unity | `019a8c0` | `docs/integration/UNITY-GUIA-INTEGRACAO.md`: plano 1.0.0 → 1.5.0 para o projeto Unity, esboço C#, checklist e prompt pronto para o agente de lá | — |
| Histórico | `59cb73e` | topologia e conexão do host no histórico (migration `session/004`); **compose monta migrations por diretório** (num clone limpo faltavam 6: session 003 e analytics 008–012) | integração (Postgres real) |
| Steam | `cccd132` | conquistas espelhadas na Steam (spec `achievements/019` + `identity/006`): campo `steamApiName`, worker no consumer com backoff, backfill | unit + integração + **ao vivo com Steam falsa** (synced, skipped, backfill) |
| Console P2P | `be48674` | hospedagem e sinal de vida no console (spec `admin-ui/091`): filtro e coluna Hospedagem, badge de heartbeat, detalhe p2p com aviso de confiança, Fleet "P2P — sem frota", selo P2P no jogo | vitest (27), build, **Playwright com mundos p2p/dedicado reais** |


## Em andamento

- Nada. Branch publicada.

## Próximo (executável sem decisão do dono)

3. Consolidar as 5 tabelas `*_daily` do analytics numa `game_activity_daily` — pede ADR (altera a ADR-0021).

## Depende do dono

- Netcode p2p no repo do jogo (NGO com transporte da Steam no modo host) — o backend já está pronto (ADR-0032).
- Virar `SESSION_AUTH_MODE=enforce` quando o jogo mandar bearer e chave de servidor.
- Publisher key e app id da Steam para validar o login contra a Steam real.

## Para o dono validar localmente

- **Build das imagens**: a VM não builda as imagens Docker (o proxy TLS da nuvem não é confiável dentro do build),
  então a demo foi validada com os binários no host. Rodar `infra/docker/gen-secrets.sh`,
  `docker compose -f infra/docker/docker-compose.yml up -d --build` e `scripts/demo-host-world.sh`.
- **Integração do cloud-save**: a VM não baixou `minio/minio:latest` (`pull access denied`). Rodar
  `go test -tags=integration ./services/cloud-save-service/...`; se falhar aí também, a imagem saiu do Docker Hub e o
  compose e o teste precisam de outra origem.
- **Com o jogo (quando a fase multiplayer voltar)**: `HOST_KILLED` com o LocalStack parado → o server deve receber
  `Internal`, reenviar com o mesmo `event_id` e, com o LocalStack de volta, o invasor ganha a conquista.
- **Túnel do Pi 5**: `infra/agones/pi5/pi5-tunnels.sh install` (pendência antiga do NEXT-STEPS).
