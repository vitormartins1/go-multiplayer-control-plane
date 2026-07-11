# Spec 047 — Drawers à direita (padrão 037) + canvas em altura total

**Status:** Superseded by 056-dynamic-creation-canvas (itens 2 e 3 — canvas em altura total e Progresso do achievement em Drawer à direita — vigentes; item 1, Drawer à direita para criação de regra, foi substituído por criação pura no canvas, sem drawer)
**Serviço:** admin-ui
**Relaciona:** specs 044 (drawer de criação de regra), 037 (editor de modos em
Drawer à direita — o padrão a seguir), spec achievements/014 (form com operador)

## Contexto

Feedback do teste: o drawer de baixo (044) "não ficou legal" — o padrão do
console é o do editor de modos multiplayer (Drawer à direita, size 560, lista
visível ao fundo). E o canvas de Event Rules deve ocupar a altura disponível
da aba, não 580px fixos.

## Comportamento

1. **Criação de regra**: o form (com operadores, spec 014) passa pro **Drawer
   à direita** (position="right", size 560 — mesmo padrão do editor de modos),
   layout vertical. Sem overlay/trap de foco: o canvas continua interativo
   (clicar em condição/event type segue adicionando ao rascunho — R4 da 044
   preservado; o rascunho no topo da coluna de regras fica visível à esquerda).
2. **Canvas em altura total**: o canvas da aba Event Rules ocupa a altura
   disponível da viewport (`calc(100vh - offset)`, com mínimo), em vez de
   580px fixos.
3. **Progresso do achievement**: o modal "Progresso — <nome>" da aba
   Achievements vira **Drawer à direita** (mesmo padrão), conteúdo intacto.

## Critérios de aceite

1. Criar regra pelo drawer direito funciona de ponta a ponta (com operadores)
   e o canvas segue clicável durante a criação.
2. Canvas de Event Rules preenche a altura da janela (sem scroll interno
   desperdiçado) em viewports comuns.
3. Progresso do achievement abre à direita, sem modal.
4. `npm run build` verde.
