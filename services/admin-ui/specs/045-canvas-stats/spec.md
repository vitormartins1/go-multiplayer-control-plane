# Spec 045 — Stats (métricas) no canvas de Event Rules

**Status:** Done (fatia pequena/aditiva — spec curta, sem plan/tasks)
**Serviço:** admin-ui (UI-only; dado já disponível nas APIs que a aba consome)
**Relaciona:** specs 043/044, ADR-0028, ADR-0023 A1 (critério por métrica)

## Contexto

Achievements por métrica (`statKey`) não passam por event rules — o progresso
é aplicado direto pela stat da partida (mutuamente exclusivo com `eventType`).
Hoje eles são invisíveis no canvas, então o grafo dá um retrato incompleto de
"o que alimenta cada conquista". Stats declaradas vivem em
`params.scoringStats` dos modos (nem todo arquétipo tem — host_world_dropin
não declara stats).

## Comportamento

- **Toggle "Stats" na toolbar** (padrão LIGADO). Ligado, o canvas ganha:
  - **Nós Stat** (read-only, visual próprio — laranja) na coluna de gatilhos
    (a mesma dos event types), com edge direta stat → achievement.
  - Os **achievements por métrica** na camada 4, ligados à sua stat.
- **Quais stats aparecem** (no arquétipo selecionado):
  1. `scoringStats` dos modos do jogo daquele arquétipo (mesmo sem achievement
     ouvindo — vocabulário declarado);
  2. `statKey` de achievements **não declarada em nenhum modo do jogo** —
     mostrada com **aviso de drift** (achievement pode nunca progredir).
  Stat declarada só em modos de OUTRO arquétipo não aparece aqui (aparece no
  canvas daquele arquétipo).
- **Painéis**: nó Stat mostra se é declarada (e que o progresso é direto, sem
  regra) ou o aviso de drift + quantos achievements progridem por ela;
  achievement por métrica mostra `statKey` + agregação (acumulada/por partida).
- **Honestidade visual**: stats não participam do draft/criação (não existe
  "regra de stat" no motor); clicar numa stat com o drawer aberto não faz nada.

## Critérios de aceite

1. Toggle liga/desliga a camada de stats sem reload da página.
2. Achievements por métrica ficam visíveis no canvas (ligados à sua stat).
3. Stat usada por achievement mas não declarada em nenhum modo aparece com
   aviso de drift no card e no painel.
4. Zero mudança de backend; `npm run build` verde.
