# Spec: 028-rooms-mode-badge

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-07)
**Data:** 2026-07-07
**Parte de:** lote console-polish. Surfacing do M1 (ADR-0024): o board de sessões já carrega
`modeId` (session/017 → BFF), a UI ainda não o exibe.

## Comportamento

- THE SYSTEM SHALL exibir em cada sessão do Rooms (e da aba Sessions do jogo) um badge com o
  `modeId` (variant light). Sessão sem o campo (legado) → badge `default`.
- WHEN houver mais de um `modeId` distinto nas sessões listadas → THE SYSTEM SHALL exibir um
  filtro (Select "Todos os modos" + valores presentes) que restringe a lista client-side.
- O filtro não aparece com 0–1 modos distintos (sem ruído no caso comum).

## Contratos de Interface (cliente)

- `src/lib/api.ts`: campo `modeId?: string` no tipo da sessão do board (aditivo — conferir o
  nome exato no JSON do BFF na implementação).
- `src/pages/Rooms.tsx` e `src/pages/GameSessions.tsx`: badge + filtro.

## Casos de Teste / Verificação

- [x] sessão criada com modo → badge com o modeId; resposta sem campo → `default`
- [x] filtro aparece só com ≥2 modos e restringe corretamente
- [x] `npm run build` limpo

## Fora do escopo

- Filtro server-side; segmentação de analytics por modo (M2, gated).
