# Spec: 075-mode-scoring-stats-visible-in-table

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071/072/073/074.

## Contexto

Pedido do usuário na revisão da aba Configurações: os `scoringStats` de
cada modo (as métricas extras que o data plane reporta — kills, assists,
etc.) ficam "meio escondidas" apesar de serem importantes — elas
alimentam o event system que produz achievements (critério "por
métrica"), leaderboards (um board automático por stat) e analytics
(stats agregados).

Antes desta spec, `scoringStats` só aparecia em dois lugares:
1. Dentro do form de edição de cada modo (Drawer, `TagsInput` na seção
   "Métricas") — só visível clicando em "Editar".
2. No grafo "Pipeline de dados" (`GamePipeline.tsx`, mais abaixo na
   página, dentro de cada card `MODO`) — já era uma tentativa anterior
   de resolver o mesmo problema (comentário original do componente: "tira
   as scoringStats do esconderijo do form"), mas exige rolar a página e
   ler um grafo pra correlacionar stat com modo.

A tabela principal "Modos de multiplayer" — o primeiro lugar que
qualquer um olha pra entender o que está cadastrado — nunca mostrava
scoringStats; a coluna "Configuração" (`modeSummary()`) só resume
parâmetros estruturais (composição de times, condição de vitória,
papéis, lobby, missões).

## Decisões

- `modeScoringStats(m)` extraído de dentro de `buildPipelineGraph`
  (`lib/pipelineGraph.ts`) pra função exportada — mesma extração,
  reusada agora por 2 lugares (o grafo e a tabela), sem duplicar lógica.
- Tabela de modos (`MultiplayerSettings.tsx`): a coluna "Configuração"
  ganha uma segunda linha, abaixo do resumo estrutural — um `Badge`
  laranja por stat declarada. Cor laranja = mesma convenção já usada
  pelo badge de critério "por métrica" no catálogo de conquistas
  (`GameAchievements.tsx`), não uma cor nova.
- Sem stats declaradas: nenhum badge aparece, sem placeholder — mesmo
  padrão de "campo aditivo vazio" usado em outros lugares do console.

## Fora de escopo

- Mudar o grafo Pipeline de dados (continua mostrando stats do jeito
  que já mostrava — a tabela e o grafo são visões complementares, não
  duplicação a resolver).
- Tornar os badges clicáveis/navegáveis (ex.: pular pro board do
  leaderboard daquele stat) — melhoria futura, não pedida agora.

## Addendum: limite de largura + "+N" (achado em uso real, mesmo dia)

Um modo `host_world_dropin` com 16 `scoringStats` (jogo "Achievements
Test", modo "Mundo Vivo") estourava a célula "Configuração" — a tabela é
auto-layout, então o conteúdo mais largo dita a coluna: ID quebrava em 2
linhas, o badge de Arquétipo truncava com "…". Correção:

- `<Stack maw={320}>` ao redor do conteúdo da célula — o wrap acontece
  DENTRO da célula (linha cresce em altura), não mais roubando largura
  das colunas vizinhas.
- Acima de `STATS_VISIBLE = 6` stats, corta e agrupa o resto num badge
  `+N` (`variant="outline"`, cinza — visualmente distinto dos badges de
  stat reais) com `Tooltip` listando o restante.
- Jogos com poucas stats (≤6, o caso comum) não mudam nada visualmente.

## Critérios de Aceite

1. Jogo com modos que declaram `scoringStats` -> a tabela "Modos de
   multiplayer" mostra os stats como badges, sem precisar abrir o form
   de edição.
2. Modo sem `scoringStats` -> célula só com o resumo estrutural, sem
   badge vazio.
3. Sem erro de console, sem regressão no grafo Pipeline de dados
   (validado com Playwright, jogo Neon Breach — 3 modos, todos com
   stats).
