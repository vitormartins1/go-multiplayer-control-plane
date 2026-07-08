# Spec: 042-player-json-ux

**Serviço:** admin-ui
**Status:** Aprovado (2026-07-08) — comportamento definido pelo usuário; detalhes de UX delegados
**Data:** 2026-07-08
**Parte de:** lote de UX da página do jogador (040). Client-only.

## Contexto

As abas Player data e Cloud saves mostram JSON como `<Code>` monoespaçado cru; a
tabela do player data tem `highlightOnHover`, que atrapalha a leitura das células
com JSON. O usuário quer interagir (expandir/colapsar, sem editar) com a estrutura
e, no cloud save, uma leitura "materializada" do conteúdo lado a lado com o JSON.

**Decisão de lib:** `react18-json-view` (tree colapsável read-only; zero deps,
tema dark nativo, API estável) — escolhida sobre `@uiw/react-json-view` (alpha).
A visão materializada é componente próprio (Mantine), sem lib.

## Comportamento

- Player data: a tabela perde o `highlightOnHover` (mantém `striped`/borda).
- Valores JSON (objeto/array) em **ambas** as abas passam a usar um **tree
  interativo read-only** (`JsonTree`): expande/colapsa chaves `{}` e arrays
  `[]`, segue o color scheme do console, sem edição. *(Refinado 2026-07-08:
  copiar valor/subárvore habilitado — interação sem edição; nó colapsado
  mostra a contagem de itens; strings longas truncam expansível.)* Primitivos
  continuam como `Code` inline.
- Player data ganha a coluna **Leitura consolidada** à direita da coluna
  Estrutura (JSON) — o mesmo `JsonMaterialized` do cloud save, por linha.
  *(Refinado 2026-07-08, a pedido do usuário após validar no cloud save.)*
- Cloud save (quando o blob é JSON): abaixo dos metadados, **duas colunas**
  (1 em telas menores):
  - **Estrutura (JSON)** — o tree interativo do blob decodificado;
  - **Leitura consolidada** — o mesmo conteúdo "materializado" para leitura
    rápida: chave/valor por linha; booleanos e arrays de escalares como badges;
    objetos aninhados como sub-seções indentadas (borda à esquerda); arrays de
    objetos como itens numerados.
- Blob texto (não-JSON) mantém o `Code` cru; binário mantém o aviso atual.

## Casos de Teste / Verificação

- [ ] player data: hover não muda mais a cor das linhas
- [ ] valor objeto/array vira tree colapsável; número/string/bool continuam Code
- [ ] cloud save JSON: duas colunas (tree + leitura consolidada) com o mesmo dado
- [ ] leitura consolidada: escalares, arrays de escalares (badges), objeto
      aninhado (sub-seção) e array de objetos (numerado) renderizam bem
- [ ] save texto/binário: comportamento anterior preservado
- [ ] `npm run build` limpo

## Fora do escopo

- Edição de player data/save pelo console; busca dentro do JSON; download do blob.
