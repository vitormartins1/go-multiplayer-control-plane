# Spec: 024-avatar-by-login-kind

**Serviço:** admin-ui
**Status:** Draft
**Data:** 2026-07-07
**Parte de:** lote console-polish (revisão de UX pós-M1).

## Contexto

Todos os jogadores usam o mesmo estilo DiceBear (`adventurer`) no `PlayerAvatar`. O identity
gera guests com subject `guest-<uuid>` (`identity/service/auth_service.go`) — o prefixo do
`playerId` distingue login anônimo de conta registrada, sem consulta extra. Esta fatia troca
o estilo por tipo de login e adiciona um badge "guest" nas listagens.

## Comportamento

- THE SYSTEM SHALL renderizar o `PlayerAvatar` com o estilo **Bottts** quando a seed começar
  com `guest-`, e **Open Peeps** caso contrário (ambos de Pablo Stanley, já presentes em
  `@dicebear/styles`). Determinístico: mesma seed → mesmo avatar em todas as páginas.
- THE SYSTEM SHALL manter a paleta de fundo determinística atual nos dois estilos.
- WHEN uma linha de jogador com `guest-` aparecer em GamePlayers, GameSessions e Rooms →
  THE SYSTEM SHALL exibir um badge `guest` (variant light, cor neutra) ao lado do handle/ID.
  (Players lista só contas registradas — nenhum badge esperado lá.)

## Contratos de Interface (cliente)

- `src/components/PlayerAvatar.tsx`: importa `bottts.json` + `open-peeps.json`; escolhe o
  `Style` pela seed. `adventurer` sai.
- `src/components/GuestBadge.tsx` (ou helper `isGuest(id)` em `lib/`): badge condicional
  reutilizado nas 3 páginas.

## Casos de Teste / Verificação

- [ ] seed `guest-...` → robô (Bottts); seed uuid de conta → Open Peeps; estável entre páginas
- [ ] badge guest aparece em GamePlayers/GameSessions/Rooms para IDs `guest-`, ausente para contas
- [ ] `npm run build` (tsc + vite) limpo

## Fora do escopo

- Persistir tipo de login no backend; avatar custom por upload.
