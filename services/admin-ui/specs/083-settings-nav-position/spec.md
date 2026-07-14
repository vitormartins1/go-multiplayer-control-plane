# Spec: 083-settings-nav-position

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação (retroativa) — mesmo
caso das specs 071 a 082. O código já estava commitado referenciando
"spec 083" nos comentários; este arquivo só formaliza o que já foi feito.

## Contexto

A aba "Settings" (spec 082) entrou no fim da lista de navegação principal,
misturada com os itens de produto (Games, Players, Rooms, Analytics, Fleet,
Infrastructure). O usuário pediu para fixá-la no rodapé do navbar,
visualmente separada — preferências do console são uma categoria diferente
de conteúdo/produto, não "mais uma seção de dado".

## Decisão

`App.tsx`: `NAV` (itens de produto) e `SETTINGS_NAV` (item isolado) viraram
duas constantes separadas. `AppShell.Navbar` passou a renderizar
`<Stack h="100%" justify="space-between" gap="sm">` com dois `<Stack gap={0}>`
internos — o primeiro com os itens de `NAV`, o segundo com um `<Divider
mb="sm" />` + o `NavLink` de Settings. `justify="space-between"` empurra o
segundo `Stack` para o rodapé do navbar, com o `Divider` marcando a
transição visual.

Mesmo padrão usado por consoles comparáveis (Slack, Notion) para separar
"conteúdo" de "preferências do produto".

## Critérios de Aceite

1. "Settings" aparece fixo no rodapé do navbar, com um `Divider` acima,
   visualmente separado dos demais itens — confirmado via screenshot
   desktop.
2. Mesmo comportamento no drawer mobile (`Burger` + navbar colapsado) —
   confirmado via screenshot com viewport estreito.
3. Navegação e estado `active` do item continuam funcionando normalmente.

## Fora de escopo

- Qualquer outro item novo de nav — só reposicionamento do que já existia.
