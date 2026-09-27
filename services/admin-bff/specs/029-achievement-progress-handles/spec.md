# Spec: 029-achievement-progress-handles

**Serviço:** admin-bff
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** gravação dos vídeos de feature (o progresso de uma conquista listava o jogador pelo UUID).
Fatia pequena/aditiva: mesmo enriquecimento de handle da spec 088 (leaderboard) e das sessões.

## Comportamento

- `GET /api/games/{gameId}/achievements/{id}/progress` SHALL devolver `handle` em cada entrada, resolvido no
  identity pelo `playerId`, best-effort (mesmo `handlesByID` do leaderboard e das sessões).
- IF o identity estiver fora ou não conhecer o jogador, THEN a entrada SHALL vir sem `handle` e a resposta SHALL
  continuar 200.

## Contratos de Interface

```go
// internal/domain/admin.go
type AchievementProgressEntry struct {
    // ... campos existentes
    Handle string `json:"handle,omitempty"`
}
```

## Critérios de Aceite

1. Com o identity no ar, cada entrada traz o handle do jogador.
2. Com o identity fora, as entradas vêm sem handle e sem erro.
