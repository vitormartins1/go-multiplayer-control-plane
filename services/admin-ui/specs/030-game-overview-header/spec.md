# Spec: 030-game-overview-header

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-07)
**Data:** 2026-07-07
**Parte de:** lote console-insights. Mini-dashboard do jogo **no header do GameDetail** — não
uma aba nova: as abas já carregam contagens nos badges; o header agrega a fotografia do jogo
no primeiro olhar, sem duplicar navegação.

## Contexto

O header do GameDetail mostra só breadcrumb + nome. Os dados do dashboard já são carregados
pela própria página (catalog, gamePlayers, sessions) — falta agregá-los visualmente e expor
os modos (hoje invisíveis fora de Configurações).

## Comportamento

- THE SYSTEM SHALL exibir no header do GameDetail, ao lado do título: a capa do jogo
  (`GameCover` mini) + o slug esmaecido.
- THE SYSTEM SHALL exibir, sob o título, os **badges dos modos** do jogo (via
  `listMultiplayerModes`): `modeId` com a cor do arquétipo (mesmo mapa da lista de modos).
  Sem modos → badge único "sem multiplayer" (cinza). Informativo, sem navegação.
- THE SYSTEM SHALL exibir uma faixa de 4 `StatCard` (SimpleGrid responsivo) com o que a
  página já carrega: **Modos** (nº), **Salas abertas** (sessions.active), **Jogadores**
  (registrados no jogo), **Conquistas** (catálogo). Valor ausente (serviço fora) → "—".
- A faixa não introduz polling novo (usa os fetches existentes; modos = 1 fetch novo único).

## Contratos de Interface (cliente)

- `GameDetail.tsx`: useAsync de `listMultiplayerModes`; Group do header (capa + título +
  slug); linha de badges; SimpleGrid de StatCards entre o header e as Tabs.
- `ARCHETYPE_META` (cores por arquétipo) sai de `MultiplayerSettings.tsx` para um módulo
  compartilhável (`lib/` ou `components/`) — evita import de página para página.

## Casos de Teste / Verificação

- [x] jogo com 2+ modos → badges com modeId e cor do arquétipo; sem modos → "sem multiplayer"
- [x] cards refletem os mesmos números dos badges das abas; serviço fora → "—" (sem quebrar)
- [x] capa + slug no header sem deslocar as tabs em telas estreitas
- [x] `npm run build` limpo

## Fora do escopo

- Aba Overview dedicada; gráficos no header; navegação por clique nos badges de modo;
  polling adicional.

## Ajuste pós-validação visual (2026-07-07)

Feedback do Vitor: os 4 StatCards eram redundantes com os badges das tabs — removidos
(o componente `StatCard` continua em uso em outras páginas). Os badges de modos subiram
para **a mesma linha do nome** (capa mini + título + badges), no lugar do slug — o slug
fica na aba Configurações. Sem fetch novo além do `listMultiplayerModes` já existente.
