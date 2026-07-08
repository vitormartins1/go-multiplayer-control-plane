# Spec: 036-settings-cover-inline

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** lote console-ux-2. Substitui o layout da 026 (coluna à direita) — terceira
iteração da capa nas Configurações, agora com a solução estrutural (ideia do Vitor).

## Contexto

A coluna dedicada à direita (026, ajustada 2×) nunca assentou: 1/3 dominava, 1/4 ainda
deixava espaço morto à direita. Diagnóstico: a capa não precisa de uma COLUNA — ela é
conteúdo de identidade. Movê-la para dentro de "Dados do jogo" elimina a coluna, devolve
a largura toda ao conteúdo e resolve a responsividade sem sticky/breakpoint especial.

## Comportamento

- THE SYSTEM SHALL exibir a capa DENTRO do Fieldset "Dados do jogo", à esquerda;
  à direita dela, uma coluna com Nome e Slug empilhados, o botão Salvar abaixo
  e a nota de remoção por último.
- A capa usa largura fixa ~150px (9:16 → ~267px de altura, par com a altura natural
  da coluna de campos) e **sem a faixa de nome** (o nome está no input ao lado —
  evita o texto quebrado que motivou a 1ª iteração).
- WHEN a tela for estreita (sm-) → capa acima dos campos (wrap natural), sem coluna
  fantasma.
- A página volta a ser coluna única (`Stack`, maw ~900): Identidade e Multiplayer usam
  a largura toda — sem Grid, sem sticky, sem espaço sobrando à direita.

## Contratos de Interface (cliente)

- `GameCover.tsx`: prop nova `withName?: boolean` (default true — nada muda nos outros
  usos); `false` esconde a faixa.
- `GameSettings.tsx`: remove Grid 9/3; Fieldset com `Group align="flex-start"` (capa +
  Stack de campos).

## Casos de Teste / Verificação

- [x] capa dentro de Dados do jogo, sem faixa, sem quebra de texto
- [x] Multiplayer ocupa a largura toda; nenhum espaço morto à direita
- [x] tela estreita: capa acima dos campos, formulário íntegro
- [x] `npm run build` limpo

## Fora do escopo

- Upload de capa própria; capa no header do editor de modos.
