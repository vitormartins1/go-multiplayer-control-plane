# Spec 051 — Suprimir/restaurar regra global no canvas (V2b)

> Status: **implementada** (aprovada e validada por smoke em 2026-07-10)
> Fatia: UI substancial — depende de `achievements-service/017` (endpoints
> de suppress/unsuppress + campo `suppressed`).
> Origem: mesma pergunta do ADR-0027/0028 — hoje a redundância entre regras
> só aparece como warning textual no painel; a supressão precisa da mesma
> visibilidade relacional que o resto do canvas já dá a fontes/regras/types.

## Contexto

O canvas (specs 043→050) já distingue regra global de regra do jogo
visualmente (badge, cor) e trata globais como somente-leitura no painel de
detalhes. Com `achievements-service/017`, um jogo passa a poder desligar uma
regra global só para si — o canvas precisa expor essa ação e, principalmente,
tornar o efeito **visível no flow**: uma regra suprimida não deve se
confundir com uma regra ativa (nem no nó, nem nas edges que saem dela).

**Escopo simplificador:** o console é sempre navegado por jogo
(`/games/{gameId}/event-rules`) — nunca há dois jogos no mesmo canvas. Logo
"quem suprimiu" é sempre "este jogo": não há necessidade de resolver/exibir
nome de outro jogo nem de consultar outro serviço. A indicação vira um texto
fixo no painel ("suprimida para este jogo"), sem timestamp — se depois se
revelar necessário auditar quando/quem, isso é aditivo (o backend já grava
`created_at`; expor fica para quando houver pedido concreto).

## Comportamento (EARS)

### API client

1. `EventRule` (lib/api.ts) ganha `suppressed: boolean`. Novas chamadas
   `api.suppressEventRule(gameId, ruleId)` (`POST .../event-rules/{id}/suppress`)
   e `api.unsuppressEventRule(gameId, ruleId)` (`DELETE` na mesma rota).

### Canvas — nó da regra

2. **Quando** uma regra global tiver `suppressed: true`, o nó DEVE renderizar
   esmaecido (opacidade reduzida, borda tracejada — classe
   `flow-card--suppressed` nova em `flow.css`, complementar à
   `flow-card--rule-global` existente) e o badge "read-only" DEVE virar
   "suprimida" (cor de alerta, ícone de olho cortado/proibido).
3. **Quando** uma regra global suprimida tiver edges pra event types (as
   ligações persistidas — a regra continua "existindo", só não contribui),
   essas edges DEVEM renderizar tracejadas/esmaecidas (classe
   `is-suppressed-edge` nova, mesmo padrão de `is-draft-edge`/
   `is-metric-edge` já existentes) — sinaliza visualmente "esta regra não
   está no caminho ativo", sem escondê-la do grafo (o usuário precisa achá-la
   pra restaurar).

### Painel de detalhes (NodeDetails)

4. **Quando** o nó selecionado for uma regra global NÃO suprimida, o painel
   DEVE manter o texto atual ("regra global... somente leitura") e ganhar um
   botão **"Suprimir para este jogo"** (cor de alerta, ícone consistente com
   o badge do nó).
5. **Quando** for uma regra global SUPRIMIDA, o painel DEVE trocar o texto
   para "Suprimida — não contribui para as avaliações deste jogo." e mostrar
   um botão **"Restaurar"** em vez do de suprimir.
6. **Quando** clicar em qualquer um dos dois botões, a UI DEVE chamar o
   endpoint correspondente e recarregar as regras (mesmo padrão de
   `reloadAll()` usado após delete/create/update) — sem modal de confirmação
   (ação reversível com um clique, diferente do delete de override, que é
   destrutivo).

### Redundância (ajuste de cálculo existente)

7. O cálculo de `unconditionalTypes` (event types emitidos incondicionalmente,
   usado no warning "⚠ já é emitido incondicionalmente por outra regra") DEVE
   ignorar regras suprimidas — uma regra suprimida não emite nada, então não
   pode tornar outra regra "redundante".

### Visão em tabela (paridade)

8. A tabela (`GameEventRules.tsx`, view alternativa do mesmo dado — ADR-0028)
   DEVE ganhar a mesma ação na coluna de ações para linhas globais (hoje
   vazia): botão "Suprimir"/"Restaurar" conforme `r.suppressed`, e a linha
   DEVE indicar visualmente a supressão (ex.: badge extra ou texto dimmed),
   consistente com o canvas.

## Critérios de aceite

- [x] Regra global ativa → botão "Suprimir para este jogo" no painel; clicar
      esmaece o nó, tracejam as edges de saída, badge vira "suprimida".
- [x] Regra global suprimida → botão "Restaurar" no painel; clicar devolve o
      nó ao estado normal (badge "read-only", edges sólidas).
- [x] Warning de redundância não aparece mais citando um event type que só é
      emitido incondicionalmente por uma regra agora suprimida (verificado
      por inspeção do filtro `unconditionalTypes`; a seed atual não tem
      overlap de event type entre regra incondicional e condicional do mesmo
      arquétipo pra demonstrar ao vivo sem dado de teste extra).
- [x] Ação replicada na visão em tabela, com o mesmo efeito.
- [x] Regra de jogo (override) nunca mostra ação de suprimir (só a de
      remover, já existente).
- [x] Build verde + smoke Playwright: suprimir uma regra global de teste,
      confirmar visual + painel, restaurar, confirmar volta ao normal.

## Fora de escopo

- Timestamp/auditoria de quando a supressão ocorreu (backend grava, UI não
  expõe por ora).
- V2c (prioridade) e qualquer coisa relacionada a mais de um jogo por tela.
