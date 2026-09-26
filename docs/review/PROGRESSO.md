# Progresso da revisão de produto (2026-09)

> Nota de continuidade da sessão autônoma (`docs/prompts/PROMPT-REVISAO-PRODUTO.md`). Revisão completa:
> [`REVISAO-2026-09.md`](REVISAO-2026-09.md). Atualizada a cada item.

**Branch:** `claude/revisao-produto-l2k7hr` (o prompt sugere `review/produto-2026-09`; a sessão na nuvem só
permite push para esta branch). Nada vai para a `main` — o dono revisa e mergeia.

## Feito

| Item | Commit | Nota |
|---|---|---|
| Fase 1 — revisão | (este commit) | `REVISAO-2026-09.md` |

## Em andamento

- E1 — `main` verde (teste do admin-bff + testes de data fixa do analytics).

## Próximo

E2 → E3 → E4 → E5 → E6 → E7 (ver §5 da revisão).

## Para o dono validar localmente

- Integração do cloud-save: `go test -tags=integration ./services/cloud-save-service/...` — a VM não baixou
  `minio/minio:latest`. Se falhar aí também, a imagem saiu do Docker Hub e o compose (`infra/docker/docker-compose.yml:401`)
  precisa de outra origem.
