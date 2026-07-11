# Spec 057 — prévia de regras/fonte no catálogo de achievements

**Serviço:** admin-ui
**Status:** Done
**Relaciona:** spec 055 (canvas focado por regra), spec 056 (criação dinâmica)
**Depende de:** nada novo — `GameAchievements.tsx` já carrega `eventRules`/`catalog`/`modes`; reaproveita `buildRulesGraph`/`pathHighlight`/`repackVisible` (`rulesGraph.ts`) e `FlowCanvas`.

## Contexto

Hoje, pra saber "quais regras alimentam esse achievement", o único caminho é ir na aba Event Rules e procurar manualmente. A lista de achievements (spec da coluna Critério, sessão anterior) mostra a tag do critério, mas nada além disso. Ideia do Vitor: clicar na tag expande a linha (mesmo padrão de `GameSessions.tsx`, `expandedId` + `Collapse`) e mostra uma prévia focada — o mesmo "subgrafo" que apareceria no canvas de Event Rules se você selecionasse a regra relevante, mas **só como visualização, sem editar ali**.

Dois casos, tratados diferente:
- **Achievement por evento** (`a.eventType` truthy): a prévia mostra a(s) regra(s) que emitem esse tipo + as fontes delas + os achievements que também escutam (o mesmo conjunto que `pathHighlight` a partir do nó do event type já calcula). Ganha um botão "editar regra"/"ver regra" que navega pra aba Event Rules com a regra já focada — em modo de edição se for do jogo, só focada (read-only) se for global.
- **Achievement por métrica** (`a.statKey` truthy): a prévia mostra só a fonte do stat conectada direto ao achievement (sem regra envolvida — o critério é editado no próprio cadastro do achievement, não em Event Rules). Sem botão de navegação — é só visualização.

## Comportamento (EARS)

1. **QUANDO** o usuário clica na tag de critério de um achievement na tabela, **O SISTEMA DEVE** expandir a linha (mesmo padrão de `expandedId`/`Collapse` já usado em `GameSessions.tsx`) mostrando uma prévia em canvas — sem toolbar, sem afordances de edição (não é o `GameEventRulesCanvas` completo).
2. **QUANDO** o achievement é por evento, **O SISTEMA DEVE** popular a prévia com o resultado de `pathHighlight` a partir do nó do event type (`t:${eventType}`) — regra(s) que emitem esse tipo, fontes dessas regras, e achievements que também escutam o mesmo tipo.
3. **QUANDO** o achievement é por métrica, **O SISTEMA DEVE** popular a prévia só com a fonte do stat e o próprio achievement (`pathHighlight` a partir do nó do achievement, `a:${id}`) — sem buscar regras (não existe regra envolvida nesse critério).
4. **QUANDO** o achievement por evento tem exatamente uma regra do jogo (`scope: "game"`) associada, **O SISTEMA DEVE** exibir um botão "Editar regra" que: (a) muda a aba ativa de `GameDetail` pra "Event Rules", (b) foca a regra na lista/canvas de lá, (c) abre automaticamente o formulário de edição dela.
5. **QUANDO** a regra associada é global (`scope: "global"`), **O SISTEMA DEVE** exibir um botão "Ver regra" com o mesmo efeito de navegação, mas sem entrar em modo de edição (a regra é read-only lá também).
6. **QUANDO** o achievement por evento é alimentado por múltiplas regras (raro, mas possível), **O SISTEMA NÃO PRECISA** desambiguar automaticamente — o botão de navegação foca a **primeira** regra do jogo encontrada (ou a primeira global, se não houver nenhuma do jogo); as demais continuam visíveis na prévia, só não são o alvo direto do botão.
7. **O SISTEMA NÃO DEVE** permitir nenhuma edição dentro do expander — nem do achievement, nem da regra, nem de facts/sources. Toda edição acontece nas telas que já existem (formulário de achievement nesta mesma página; Event Rules pra regras).
8. **QUANDO** o usuário clica na tag de critério de novo (ou em outra linha), **O SISTEMA DEVE** colapsar a prévia (toggle, mesmo comportamento de `expandedId` — só uma linha expandida por vez).

## Critérios de aceite

- Jogo "Achievements Test": clicar num achievement por evento (ex.: `coop_boss_helped`) expande e mostra a regra `event=BOSS_DEFEATED, outcome=SUCCESS, role=coop` + os 3 facts dela + o achievement clicado (e outros que escutem o mesmo tipo, se houver).
- Clicar em "Editar regra" (regra do jogo) → muda pra aba Event Rules, regra focada, formulário de edição já aberto.
- Clicar em "Ver regra" (regra global) → muda pra aba Event Rules, regra focada, sem modo de edição.
- Achievement por métrica (ex.: `stat.kills`): expande mostrando só a fonte do stat + o achievement — sem botão de navegação.
- Nenhum controle de edição visível dentro do expander em nenhum dos dois casos.
- `tsc --noEmit` + `npm run build` limpos; smoke manual via Playwright confirma os itens acima.

## Fora de escopo

- Edição inline no expander (avaliado, descartado — replica UX que já existe em Event Rules com mais espaço e menos fricção; ver conversa que motivou esta spec).
- Desambiguação de múltiplas regras associadas ao mesmo event type (item 6 — fica com o comportamento simples "primeira encontrada" até virar um problema real).
- Mudança de dado/API — 100% client-side, reaproveitando dados já carregados.
