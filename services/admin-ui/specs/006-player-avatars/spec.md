# Spec: 006-player-avatars

**App:** admin-ui (front-end)
**Status:** Implementada (2026-06-30)
**Data:** 2026-06-30

## Contexto

Mesma pegada da capa de jogo (003-generated-cover), agora para jogadores: cada player ganha um **avatar determinístico** gerado no cliente (DiceBear), sem salvar nada no backend. Estilo **humanoide** (`adventurer`) para parecer "fotinha" de personagem. Seed = **`playerId`** (estável e único; não muda se o handle mudar).

## Comportamento

- THE admin-ui SHALL exibir um avatar (quadradinho) ao lado do jogador na **listagem de Players** (base do jogo ativo) e no **detalhe do jogador**.
- O avatar é gerado por DiceBear (`adventurer`) a partir do `playerId`; mesmo `playerId` → mesmo avatar; ids diferentes → avatares diferentes.
- Sem persistência, sem campo novo no backend (puramente client-side).

## Contratos
- `src/components/PlayerAvatar.tsx` — `{ seed: string; size?: number }` → DiceBear `adventurer` (data-URI SVG, `useMemo`), tile arredondado.
- `pages/Players.tsx` — avatar na célula do handle.
- `pages/PlayerDetail.tsx` — avatar no cabeçalho.

## Critérios de aceitação
- [x] Listagem de players mostra um avatar por jogador (determinístico por playerId)
- [x] Detalhe do jogador mostra o mesmo avatar
- [x] `npm run build` ok
- [x] (dev) base de players semeada para visualização (sandbox 6, neon-drift 4, pixel-quest 3)

## Fora do escopo
- Estilo configurável por usuário / upload de foto
- Metadados de avatar no backend (não precisa — seed = playerId já existe)
