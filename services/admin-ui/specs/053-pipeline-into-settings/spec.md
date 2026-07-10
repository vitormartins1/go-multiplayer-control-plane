# Spec 053 — Mover Pipeline de Analytics pra Configurações

> Status: **implementada** (pedida e aprovada por descrição direta do Vitor, 2026-07-10)
> Fatia: pequena/aditiva — só admin-ui, reorganização de onde um componente
> já existente é montado; sem mudança de dado/contrato.

## Contexto

A view "Pipeline" (spec 046 — mapa modos→envelope→consumers, read-only)
vivia como um toggle dentro da aba Analytics ("Números" | "Pipeline").
Feedback do Vitor: ela é sobre **configuração** (o que o jogo tem
cadastrado — modos, e pra onde cada envelope flui) e não sobre **métricas**
(números que só existem depois que partidas acontecem) — faz mais sentido
morar em Configurações, logo abaixo da listagem de modos multiplayer, onde
a pessoa já está cadastrando/revisando os modos e ganha a visão do fluxo à
medida que cadastra.

`GamePipeline` já era auto-suficiente (só precisa de `gameId` — busca
modos/regras/catálogo sozinho), então mover é só trocar de onde ele é
montado, sem tocar no componente em si.

## Comportamento (EARS)

1. A aba **Configurações** DEVE mostrar, logo abaixo da seção "Multiplayer"
   (listagem/CRUD de modos, `MultiplayerSettings`), uma subseção "Pipeline de
   dados" com o componente `GamePipeline` — mesmo grafo/painel de sempre.
2. A aba **Analytics** DEVE perder o toggle "Números"/"Pipeline" — só resta a
   view de números (agregados), sempre visível, sem controle de alternância.
3. Nenhuma mudança de comportamento DENTRO do `GamePipeline` (mesmo fetch,
   mesmo grafo, mesmo painel de detalhes).

## Critérios de aceite

- [x] Configurações → Multiplayer → logo abaixo da lista de modos aparece
      "Pipeline de dados" com o grafo.
- [x] Analytics não tem mais o SegmentedControl; só mostra os números.
- [x] Build verde; smoke visual confirmando o novo local.

## Fora de escopo

- Qualquer mudança visual/funcional dentro do próprio `GamePipeline`.
