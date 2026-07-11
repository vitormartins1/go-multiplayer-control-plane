# Spec: 024-close-room-host-world-fix
**Serviço:** session-service (+ ajuste passivo em admin-ui)
**Status:** Implementado (código pronto, testes + smoke ao vivo ok; confirmação visual do usuário pendente)
**Data:** 2026-07-11

## Contexto

Achado ao validar a spec 060 do admin-ui (aba Rooms): o force-close do operador
(`POST /admin/sessions/{matchId}/close`) resolve a sessão via `matchIndex`, um
índice reverso `matchID -> sessionID` que só é populado por `CreateSession`
(caminho "match": queue_arena/coop_mission/battle_royale). Sessões
`host_world_dropin` (abertas por `OpenWorld`) nunca ganham `matchId` — a
identidade externa delas já é o `sessionId` (é o que `CloseWorld`/`LeaveWorld`
usam). Resultado: o botão "Encerrar" da aba Rooms, ao mandar
`matchId=""` pra uma sessão host_world, gera uma rota `/api/rooms//close`
que nunca resolve — a sala nunca fecha, sem erro claro pro operador.

## Comportamento

### Force-close por operador (admin console)
- WHEN o operador aciona o force-close de uma sala de origem "match" (tem
  `matchId`) -> THE SYSTEM SHALL encerrá-la como hoje (`CloseReason=admin`).
- WHEN o operador aciona o force-close de uma sala de origem "host_world"
  (sem `matchId`, identidade = `sessionId`) -> THE SYSTEM SHALL encerrá-la do
  mesmo jeito (`CloseReason=admin`), sem depender do `matchIndex`.
- IF o identificador enviado não corresponde a nenhum `matchId` nem
  `sessionId` de sessão ativa -> retorna `ErrSessionNotFound` (404), como já
  ocorre hoje.
- THE SYSTEM SHALL continuar liberando o server (`allocator.Release`)
  independente da origem da sessão fechada.

### Admin console (admin-ui)
- WHEN a sessão listada não tem `matchId` (host_world) -> THE SYSTEM SHALL
  enviar o `sessionId` como identificador do force-close.
- WHEN a sessão listada tem `matchId` (match) -> comportamento atual mantido
  (envia `matchId`).

## Contratos de Interface

### Service (Port de entrada) — session-service
- `CloseByAdmin(ctx, id string) (*domain.Session, error)` — `id` passa a
  aceitar `matchID` OU `sessionID`: resolve `matchIndex` primeiro (caminho
  legado), cai para lookup direto por `sessionID` se não achar.

### HTTP admin handler
- `POST /admin/sessions/{id}/close` — sem mudança de shape HTTP; só muda o
  que a implementação aceita como válido no path (mesmo formato, string
  opaca — hoje só um UUID de `matchId` "vinga"; depois, UUID de `matchId`
  OU de `sessionId`).

## Modelo de Dados

N/A — sem campo novo, só resolução de identidade existente.

## Regras de Validação

| Campo    | Regra                                                          | Erro                      |
|----------|-----------------------------------------------------------------|----------------------------|
| id (path)| deve casar com `matchIndex[id]` OU `sessions[id]` de uma sessão ativa | 404 `not_found` (`ErrSessionNotFound`) |

## Casos de Teste Obrigatórios

- [ ] Force-close de sessão queue_arena (por `matchId`) continua funcionando — regressão
- [ ] Force-close de sessão host_world_dropin (por `sessionId`) passa a funcionar — bug corrigido
- [ ] Force-close de id desconhecido (nem `matchId` nem `sessionId`) retorna 404
- [ ] admin-ui: linha sem `matchId` (host_world) dispara o close usando `sessionId`
- [ ] admin-ui: linha com `matchId` dispara o close usando `matchId` (comportamento atual preservado)
- [ ] `go test ./services/session-service/...` e `tsc --noEmit`/`npm run build` do admin-ui limpos

## Fora do escopo

- Unificar a identidade externa das sessões (expor sempre um único `id` em
  vez de `matchId`/`sessionId` separados) — mudança de contrato maior que o
  bug em questão; aqui só se resolve pelo lado do resolvedor de identidade no
  force-close administrativo.
- Renomear o path param `{matchId}` pra `{id}` — cosmético, não afeta o wire;
  fora do escopo desta correção (ver plan.md).
- Outros usos hipotéticos de `matchId` vazio fora do force-close — não
  encontrado nenhum outro na varredura desta spec.
