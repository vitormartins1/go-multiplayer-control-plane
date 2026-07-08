# ADR-0025: Remoção de jogo — soft delete no game-registry

**Status:** Accepted
**Data:** 2026-07-08
**Serviço(s) afetado(s):** game-registry, admin-bff, admin-ui (demais serviços: zero mudança)
**Relaciona:** ADR-0017 (game-registry como fonte canônica), ADR-0019 (presença por jogo), ADR-0024 (snapshot do perfil por sessão — pré-requisito que torna o corte barato)

---

## Contexto

O console tem rename de jogo desde a spec `game-registry/005`, mas a aba Configurações avisa que "remover ainda não está disponível". O bloqueio nunca foi o DELETE em si — é o **dado órfão**: o dado por jogo está espalhado em **6 stores de 4 tecnologias**, sem transação possível entre eles:

| Serviço | Store | Dado por jogo |
|---|---|---|
| game-registry | Postgres | `games` (canônico, modos JSONB), `game_players` (presença, FK) |
| achievements | Postgres | catálogo, progresso, `processed_matches`/`processed_participations` |
| player-data | Postgres | `player_data` com PK por `game_id` |
| leaderboard | Redis | sorted sets `<prefix>:<gameID>:<board>` + set de idempotência |
| cloud-save | S3/MinIO | objetos com prefixo `{gameID}/` |
| analytics | Postgres | 5+ tabelas de rollup por `game_id` |

Dois fatos mudaram a economia da decisão desde que o delete foi adiado:

1. **Matchmaking e session já dependem do registry a cada fluxo** (ADR-0024/M1): o matchmaking consulta o perfil via `profilecache` (TTL 15s) e o session **falha o `Create`/`OpenWorld` se o registry recusar** o snapshot. Se o registry responder 404 para um jogo deletado, fila e criação de sessão morrem sozinhas — sem mudança em nenhum outro serviço.
2. **O console é jogo-cêntrico**: toda navegação até dado satélite (conquistas, saves, boards, rollups) passa pela biblioteca de jogos. Sumir da listagem = sumir de tudo.

## Alternativas Consideradas

### Opção A: **Soft delete no registry** (escolhida)
- **Prós:** corte em **um serviço só** — coluna `deleted_at` em `games`; jogo deletado some das listagens, `GET /games/{id}` → 404, mutações → 404, presença recusada. Matchmaking/session param sozinhos (fato 1 do contexto). Reversível por SQL (auditoria preservada). Sem risco de perda em cascata por clique errado.
- **Contras:** dado satélite fica órfão ocupando storage (irrelevante em dev/tese); jogo deletado é indistinguível de "nunca existiu" para clientes (404, não 410 — aceito: menor superfície).

### Opção B: Hard cascade (purge cross-service síncrono/assíncrono)
- **Contras:** exigiria mecanismo novo — evento `game-deleted` SNS→SQS com consumidor em 5 serviços, ou endpoints internos de purge orquestrados pelo BFF — mais tratamento de **falha parcial** (purge que morre no meio deixa estado pior que o órfão), idempotência e ordem. Torna um clique de admin **irreversível** sobre 6 stores. Custo e risco altos; o benefício (liberar storage) é irrelevante no estágio atual. Rejeitada.

### Opção C: Soft delete agora + purge assíncrono depois (evolução da A)
- Um job offline varre `deleted_at < now() - N` e chama limpeza por serviço (endpoints de purge admin). Cobre demanda futura real (custo de storage, LGPD/GDPR). **Registrada como evolução, gated por evidência** — mesmo critério do M2 do ADR-0024. A Opção A não fecha essa porta.

## Decisão

Adotamos a **Opção A**, em cinco pontos:

1. **Migration**: `games` ganha `deleted_at TIMESTAMPTZ` (NULL = vivo). Nenhuma outra tabela muda.
2. **`DELETE /games/{id}`** no registry marca `deleted_at` e responde 204; deletar de novo (ou id inexistente) = 404. **Todas** as leituras e mutações do registry passam a filtrar `deleted_at IS NULL` — listagem, get, rename, modos, marca de presença (login-into-game vira no-op para jogo morto), base de players do jogo e consulta reversa "jogos do player".
3. **Sessões em andamento terminam naturalmente**: sem force-close. `ReportResult` de jogo deletado continua aceito pelos sinks (vira rollup órfão invisível — inofensivo). Novas filas/sessões morrem no registry (staleness máx. = TTL do profilecache, 15s).
4. **BFF**: passthrough `DELETE /api/games/{id}`. **UI**: botão "Remover jogo" na aba Configurações, com modal de confirmação **digitando o nome do jogo** (padrão para ação destrutiva).
5. **Sem undelete na API**: recuperação é `UPDATE games SET deleted_at = NULL` manual no banco. `game_id` é UUID — nunca reutilizado; o slug de um jogo deletado **permanece reservado** (a unicidade de slug não exclui deletados, evitando ambiguidade de identidade humana).

## Consequências

### Positivas
- A última lacuna funcional visível do console fecha com mudança em 1 serviço + passthrough + UI.
- Zero mudança em matchmaking, session, sinks e data planes — o M1 (ADR-0024) já pagou o custo do corte centralizado.
- Reversível e auditável (`deleted_at` preservado).

### Negativas / Trade-offs
- Storage órfão cresce a cada delete (aceito; Opção C é o escape documentado).
- Slug reservado para sempre: recriar "o mesmo jogo" exige slug novo (aceito: identidade nova de fato).
- Sessões abertas do jogo deletado seguem visíveis no board Rooms até terminarem (aceito: refletem a realidade do data plane).

### Neutras / Implicações
- O purge (Opção C) definiria endpoints internos de limpeza por serviço — só quando houver demanda real.
- `identity` não muda: contas são globais (ADR-0019); a presença do jogo morto apenas deixa de ser marcada/listada.

## Referências
- `docs/NEXT-STEPS.md` — backlog item 5 (delete adiado aguardando esta decisão).
- ADR-0017, ADR-0019, ADR-0024 (snapshot por sessão + profilecache — fato que barateou a Opção A).
- Specs desta decisão: `game-registry/010-game-soft-delete`, `admin-bff/018-game-delete-passthrough`, `admin-ui/038-game-delete`.
