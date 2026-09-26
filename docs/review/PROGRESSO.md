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

## Em andamento

- Nada em aberto no código. Próximos candidatos abaixo.

## Próximo (executável sem decisão do dono)

1. P3 — TTL/janela no set de idempotência do leaderboard (precisa de spec curta: a janela é uma decisão técnica).
2. P2 — Vitest nos `lib/` puros do admin-ui + ESLint no CI.
3. P6 — `govulncheck` no CI: **não validável nesta VM** (`vuln.go.dev` bloqueado); rodar localmente antes de ligar.

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
