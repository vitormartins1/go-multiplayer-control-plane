# Spec 060 — Rooms: agrupamento por jogo + ordenação por coluna

**Serviço:** admin-ui (só frontend — sem mudança de contrato/backend)
**Status:** Draft
**Data:** 2026-07-11
**Relaciona:** admin-bff/022 (ícone de partida + capa do jogo na 1ª coluna, base visual reaproveitada aqui)

## Contexto

Hoje a aba Rooms mostra **uma tabela só**, com as sessões de todos os jogos misturadas (ordem = o que a API devolve, sem controle do operador). Motivação do Vitor: poder **agrupar por jogo** (uma seção/tabela por jogo, em vez de tudo junto) e **ordenar** — tanto na visão unificada quanto dentro de cada seção — por estado, arquétipo, modo, e o que fizer sentido além disso.

Decisões já fechadas com o usuário antes de escrever esta spec:
- Agrupar por jogo é uma **visão alternativa** (toggle), não substitui a tabela unificada — as duas convivem.
- Ordenar é por **clique no cabeçalho da coluna** (padrão comum de tabela), não um dropdown separado.
- A visão escolhida (geral/por jogo) e a ordenação ficam em **localStorage** — não resetam a cada visita.

## Comportamento (EARS)

### Alternância de visão

1. **O SISTEMA DEVE** adicionar um controle (`SegmentedControl`, mesmo componente já usado no filtro Abertas/Fechadas/Todas) com duas opções: **Geral** (tabela única, comportamento atual) e **Por jogo** (uma seção por jogo).
2. **O SISTEMA DEVE** lembrar a última visão escolhida entre sessões (localStorage) — abrir a aba Rooms de novo mantém a mesma visão da visita anterior.
3. **QUANDO** a visão é "Por jogo", **O SISTEMA DEVE** mostrar uma seção por jogo com pelo menos uma sessão no filtro atual (Abertas/Fechadas/Todas) — jogo sem sessão nenhuma no filtro atual **NÃO** aparece (evita seção vazia por padrão).
4. Cada seção **DEVE** ter um cabeçalho com a capa do jogo (`GameCover`, variante mini), nome do jogo e a contagem de sessões daquela seção no filtro atual.
5. **O SISTEMA DEVE** ordenar as seções (jogos) por número de sessões no filtro atual, decrescente — jogos com mais atividade aparecem primeiro; em caso de empate, ordem alfabética pelo nome do jogo.
6. Dentro de cada seção, a tabela **NÃO DEVE** repetir a coluna "Jogo" (redundante — já está no cabeçalho da seção); as demais colunas (Arquétipo, Modo, Match, Estado, Players, Server, Tempos, ações) continuam.

### Ordenação por coluna

7. **O SISTEMA DEVE** tornar clicáveis os cabeçalhos das colunas: **Jogo** (nome do jogo — só na visão Geral, onde a coluna existe), **Arquétipo**, **Modo**, **Estado**, **Players** (contagem) e **Tempos** (aberta em/duração) — clicar ordena por aquela coluna; clicar de novo na mesma coluna inverte a direção.
8. **O SISTEMA DEVE** indicar visualmente qual coluna está ordenando e a direção (ícone de seta no cabeçalho ativo).
9. **A ordenação é uma configuração ÚNICA**, aplicada igualmente à tabela Geral e a CADA seção da visão Por jogo (não é por-seção) — evita estado combinatório (N jogos × N critérios) sem perda prática: o operador normalmente quer "ver tudo por Estado" em qualquer visão, não uma seção por Modo e outra por Players.
10. **O SISTEMA DEVE** lembrar a última coluna/direção de ordenação escolhida entre sessões (localStorage), junto da preferência de visão (item 2).
11. **Ordem de desempate:** dentro do mesmo valor de ordenação (ex.: dois "abertas"), a ordem original da API prevalece (estável) — sem critério secundário implícito que surpreenda o operador.
12. **Estado (Estado):** a ordenação por Estado usa a ordem `open` antes de `closed` (não alfabética — "open" > "closed" alfabeticamente ficaria contra-intuitivo).
13. **Arquétipo/Modo:** ordenação alfabética pelo **rótulo visível** (`ARCHETYPE_META[...].label` pro arquétipo; o texto do `modeId` pro modo) — sessão sem arquétipo conhecido (legada) ordena para o fim, sempre, independente da direção.
14. **Players:** ordenação numérica pela contagem (`session.players.length`).
15. **Tempos:** ordenação por `openedAt` (sessão aberta) ou pela duração calculada (`closedAt - openedAt`, sessão fechada) — mistura de abertas/fechadas (filtro "Todas") ordena todas pelo mesmo valor de tempo (`openedAt` quando aberta, `closedAt` quando fechada), sem tentar comparar duração com "há quanto tempo está aberta" (unidades diferentes seria confuso).

### Coluna Arquétipo nova (Geral e Por jogo)

16. **O SISTEMA DEVE** adicionar uma coluna "Arquétipo" (hoje o ícone de arquétipo só aparece dentro do badge de Modo) — badge com ícone + rótulo (`ARCHETYPE_META`), mesmo padrão visual já usado em Configurações → Multiplayer.
17. A coluna "Modo" (hoje embutida na coluna "Jogo") **DEVE** virar uma coluna própria — badge com o `modeId`, sem o ícone de arquétipo duplicado (que já está na coluna Arquétipo nova).

## Modelo de Dados

```ts
// localStorage, uma chave só
interface RoomsViewPrefs {
  view: "unified" | "grouped";
  sortBy: "game" | "archetype" | "mode" | "state" | "players" | "time";
  sortDir: "asc" | "desc";
}
```

## Casos de Teste Obrigatórios

- [ ] Alternar Geral ↔ Por jogo preserva o filtro (Abertas/Fechadas/Todas) e a ordenação ativa
- [ ] Visão Por jogo: jogo sem sessão no filtro atual não aparece; contagem no cabeçalho da seção bate com o número de linhas
- [ ] Visão Por jogo: seções ordenadas por contagem desc, empate por nome alfabético
- [ ] Clicar num cabeçalho ordena; clicar de novo inverte; ícone de seta reflete o estado
- [ ] Ordenação por Estado: abertas sempre antes de fechadas (filtro "Todas")
- [ ] Ordenação por Arquétipo/Modo: alfabética por rótulo; sessão sem arquétipo vai para o fim nos dois sentidos
- [ ] Ordenação por Players: numérica pela contagem
- [ ] Ordenação por Tempos: `openedAt` (aberta) / `closedAt-openedAt` (fechada), sem comparar as duas unidades entre si de forma capciosa
- [ ] Reload da página mantém view/sort da visita anterior (localStorage)
- [ ] Coluna "Arquétipo" nova e coluna "Modo" separada da "Jogo", nos dois modos de visão
- [ ] `tsc --noEmit` + `npm run build` limpos

## Fora de escopo

- **Seções colapsáveis** na visão Por jogo — todas ficam sempre expandidas; se a lista de jogos crescer muito, revisar depois com evidência real de necessidade.
- **Ordenação por-seção independente** (uma seção por Estado, outra por Players) — ver item 9, decisão deliberada.
- **Filtro por jogo específico** na visão Por jogo (ex.: mostrar só 2 dos N jogos) — a visão já filtra por Abertas/Fechadas/Todas e por Modo (já existe); um filtro por jogo é aditivo, não está pedido aqui.
- **Persistir a preferência no backend/por usuário** — só localStorage local do browser (mesmo computador/perfil), sem conta de operador multi-dispositivo.
