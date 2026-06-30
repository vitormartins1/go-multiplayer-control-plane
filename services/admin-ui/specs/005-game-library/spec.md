# Spec: 005-game-library

**App:** admin-ui (front-end)
**Status:** Implementada (2026-06-30)
**Data:** 2026-06-30

## Contexto

Refina a UX multi-tenant do painel (ADR-0017). Hoje a aba **Games** é uma tabela e o jogo é re-selecionado em cada aba do jogador (um `<Select>` repetido em Achievements/Player data/Cloud saves — features 003/005/006). Esta feature torna o painel **jogo-cêntrico**:

1. **Games vira uma "biblioteca"** — grid de **capas verticais 9:16** (estilo prateleira), em vez de tabela. São poucos jogos; identificar/clicar fica imediato.
2. **Jogo ativo global** — um contexto de jogo selecionado no **header**, persistido (localStorage), compartilhado pelo app. As abas do jogador consomem esse contexto — **fim dos `<Select>` repetidos**.
3. **Capa procedural** — sem backend ainda: a capa é gerada por código (gradiente determinístico do slug + inicial do nome). O campo `coverUrl` real fica para depois.

**Fase A** de uma melhoria maior. Não toca em backend. As fases seguintes (ADR de "presença por jogo" → Players por jogo; e `coverUrl` no game-registry) são features próprias.

## Telas e navegação

- **Header (topbar):** seletor do **jogo ativo** (capa-mini + nome, dropdown para trocar). É o contexto compartilhado. Persistido entre reloads.
- **Games:** biblioteca de capas (grid responsivo). Clicar numa capa **seleciona o jogo ativo** e abre o detalhe (`/games/:id`). "Registrar jogo" e "Atualizar" mantidos.
- **Player detail:** as abas (Achievements, Player data, Cloud saves) usam o **jogo ativo do header** — sem seletor próprio. Sem jogo ativo → orienta a escolher no topo.

## Comportamento (UX)

- WHEN o app carrega → `GET /api/games` (uma vez, no contexto) popula a biblioteca e o seletor do header.
- THE APP SHALL manter um **jogo ativo** em estado global + localStorage; trocar no header reflete em todo o painel.
- WHEN abre **Games** → grid de capas (capa procedural + nome + slug). Clicar → seleciona ativo + navega ao detalhe. Registrar jogo (sucesso entra na lista e vira ativo; 409/400 com mensagem, como hoje).
- WHEN abre um **jogador** → as abas leem o jogo ativo: IF há ativo → carregam os dados do jogador naquele jogo (como hoje, sem o Select); IF não há → mensagem "Selecione um jogo no topo".
- IF a lista de jogos estiver indisponível (`502`) → estado de erro no header/Games, sem quebrar.

## Layout e visual

- **Capa (`GameCover`):** `AspectRatio` 9/16; fundo gradiente derivado do hash do slug (hue determinístico); inicial do nome grande ao centro; nome (+ slug) no rodapé. Reutilizável: tamanho "cover" (biblioteca) e "mini" (header).
- **Biblioteca:** `SimpleGrid` responsivo (ex.: 2–6 colunas conforme largura), capas clicáveis com hover.
- Tema dark, consistente com o resto.

## Critérios de aceitação

- [ ] Header tem seletor de jogo ativo (capa-mini + nome); trocar persiste e reflete no painel
- [ ] Games é uma biblioteca de capas 9:16 (procedural); clicar seleciona o ativo e abre o detalhe
- [ ] Registrar jogo segue funcionando (sucesso entra/seleciona; 409/400 com mensagem)
- [ ] Abas do jogador usam o jogo ativo (sem `<Select>` por aba); sem ativo → orienta escolher
- [ ] `npm run build` ok; estados loading/empty/erro tratados

## Fora do escopo

- **Players por jogo** (player base do jogo ativo) — depende do ADR de "presença por jogo" (Fase B)
- **`coverUrl` real** no game-registry + upload — Fase C
- **Editar/remover** jogo — incremental
