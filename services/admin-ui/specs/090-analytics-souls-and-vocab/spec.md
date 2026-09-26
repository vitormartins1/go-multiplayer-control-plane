# Spec: 090-analytics-souls-and-vocab

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-09-26
**Origem:** auditoria do Analytics e dos fluxos do console (revisão 2026-09, Fase 3)

## Contexto

O Analytics por jogo mostrava para o `host_world_dropin` só uma tabela papel × evento × desfecho — o operador do
Folclore Souls tinha de somar à mão quantas invasões aconteceram e quantas o invasor venceu. Rótulos de domínio
divergiam entre telas (motivo de fechamento em 3 versões, `FAILURE` onde o backend manda `FAIL`, "closed · host_left"
cru em inglês). O "Pipeline de dados" era um grafo com alturas de card calibradas à mão, sem o `session_closed`.

## Comportamento

- `lib/vocab.ts` é a fonte única de rótulos/cores/categorias: motivos de fechamento (os 6 do session-service, com
  categoria concluída/manual/abandonada), estado da sessão, papéis, eventos de participação, desfechos (`FAIL`).
  Analytics, Rooms, Sessions, histórico do jogador e o detalhe da sessão leem daqui.
- Analytics por jogo, seção Host World Drop-in, ganha o card **Resumo do mundo**: invasões, invasões vencidas,
  taxa de vitória do invasor, invasores abatidos, co-ops invocados, bosses derrotados com ajuda, co-ops mortos.
- O arquétipo de cada card é declarado no próprio card; o filtro duplicado por arquétipo do modo sai (com um modo
  selecionado o backend já devolve vazio para os outros arquétipos).
- "Pipeline de dados" vira tabela: modo · arquétipo · envelope · stats declaradas · consumidores, mais o que cada
  consumidor faz — incluindo o `session_closed`. Saem `pipelineGraph.ts`, `pipelineNodes.tsx` e o CSS calibrado.
- Rooms/Sessions: a coluna "Match" vira "Sessão" e mostra o id curto da sessão (vazia antes em mundos); o estado
  sai em português com o motivo traduzido; cabeçalhos ordenáveis com a mesma tipografia dos demais.

## Critérios de aceite

1. `vocab.test.ts` cobre os 6 motivos, o fallback e o `FAIL`.
2. `tsc`, vitest e build verdes; telas conferidas em navegador (Playwright) com dados reais de mundo.
