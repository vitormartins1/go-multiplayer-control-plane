# Spec: 031-avatars-everywhere

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** lote console-avatars. Estende a 024 (Bottts p/ guests, Open Peeps p/ registrados)
para as superfícies que ainda mostram o jogador só como texto.

## Contexto

O leaderboard e o modal de progresso de uma conquista ("ver status") exibem apenas o
`playerId` cru. O `PlayerAvatar` (024) já resolve o estilo pela natureza do login
(prefixo `guest-`), então basta reutilizá-lo — a identidade visual do jogador fica
consistente em todas as abas.

## Comportamento

- THE SYSTEM SHALL exibir, no leaderboard (aba Leaderboards), o `PlayerAvatar` (24px)
  à esquerda do `playerId` em cada linha, mais o `GuestBadge` quando o id for guest.
- THE SYSTEM SHALL exibir, no modal "Progresso — «conquista»" (aba Achievements), o
  `PlayerAvatar` (24px) à esquerda do jogador em cada linha da tabela, mais o
  `GuestBadge` quando aplicável.
- O avatar é derivado apenas do `playerId` (sem fetch novo, sem backend).

## Contratos de Interface (cliente)

- `GameLeaderboards.tsx`: célula "Jogador" vira `Group` (avatar + id + GuestBadge).
- `GameAchievements.tsx` (modal de progresso): idem na coluna "Jogador".

## Casos de Teste / Verificação

- [x] leaderboard com registrados e guests → Open Peeps vs Bottts lado a lado
- [x] modal de progresso mostra avatar por linha; guest ganha badge
- [x] `npm run build` limpo

## Fora do escopo

- Handle no leaderboard (a API só devolve `playerId`; enriquecer via identity é fatia
  própria, com decisão de custo no BFF); avatares em Analytics.
