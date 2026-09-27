# Spec: 069-session-health-classification

**Serviço:** admin-ui
**Status:** Done (implementada e mergeada; status corrigido na revisão 2026-09)
**Data:** 2026-07-13
**Parte de:** fechamento do follow-up "classificação de % abandono" deixado
em aberto nas specs 013 (analytics-service)/065 (admin-ui) — dado cru já
existe, falta a interpretação.

## Contexto

`SessionCloseStat` (analytics-service/013) expõe a distribuição crua por
`closeReason`, deliberadamente sem classificar o que "conta como
abandono" — decisão documentada como responsabilidade da apresentação, não
do backend. O widget "Saúde das sessões" (admin-ui/065) mostra hoje só a
tabela/donut cru. Falta a classificação em si.

Os 6 valores de `closeReason` (session-service/domain/session.go, spec 028)
têm semântica clara pra agrupar:
- `result`: partida/mundo terminou com um resultado reportado — concluída.
- `host_killed`: desfecho de jogo válido (invasor venceu) — concluída.
- `host_left`: o host escolheu sair — encerramento voluntário, NÃO é
  abandono (distinção já documentada no `CloseReason` original).
- `admin`: operador encerrou manualmente — intervenção humana, também não
  é abandono espontâneo do jogador.
- `ttl`: expirou sem nenhuma ação — sem conclusão.
- `abandoned`: o game server sumiu sem reportar (crash) — sem conclusão.

## Comportamento

- THE SYSTEM SHALL classificar cada `closeReason` numa de 3 categorias:
  **Concluída** (`result`, `host_killed`), **Encerrada manualmente**
  (`host_left`, `admin`), **Abandonada** (`ttl`, `abandoned`).
- O widget "Saúde das sessões" SHALL exibir, acima da tabela/donut crus já
  existentes (inalterados), um resumo com a % de cada categoria — 3
  `StatCard` (mesmo componente já usado no resto da página) com a
  contagem e o percentual.
- O cálculo é 100% client-side, a partir do `sessionHealth[]` já recebido
  (nenhuma mudança de API/backend) — soma por categoria / soma total.
- Quando `sessionHealth` está vazio, o widget inteiro já não aparece (
  comportamento atual, inalterado).

## Contratos de Interface

Sem mudança de tipos/API — só lógica de apresentação em cima do
`SessionCloseStat[]` já existente.

```ts
// pages/GameAnalytics.tsx
const CLOSE_REASON_CATEGORY: Record<string, "concluded" | "manual" | "abandoned"> = {
  result: "concluded",
  host_killed: "concluded",
  host_left: "manual",
  admin: "manual",
  ttl: "abandoned",
  abandoned: "abandoned",
};
```

## Critérios de Aceite

1. Jogo com os 6 motivos de fechamento presentes -> os 3 `StatCard` somam
   100% e batem com a soma manual dos `count` por categoria.
2. Jogo só com `result` -> "Concluída" 100%, as outras 2 categorias em 0%
   (aparecem, não somem — zero é informação válida).
3. Filtro por modo -> o resumo recalcula em cima do `sessionHealth`
   filtrado (já vem filtrado do backend, spec 012/013), sem lógica nova
   de filtro na classificação.

## Fora de escopo

- Mudar a tabela/donut crus já existentes — continuam mostrando o motivo
  original, não a categoria (o usuário pode querer o detalhe fino mesmo
  depois de ver o resumo).
- Tornar a classificação configurável — 3 categorias fixas, mesmo
  raciocínio de `CLOSE_REASON_LABELS`/`EVENT_LABELS` (vocabulário fixo do
  domínio, não preferência de UI).
