# Spec: 066-remove-unscoped-notice

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-07-13
**Parte de:** analytics-service/014-mode-scoped-cumulative-rollups (fatia
pequena/aditiva — remoção de código morto, sem decisão técnica nova)

## Contexto

A spec 064 introduziu `UNSCOPED_WIDGETS`/`showsUnscopedNotice`/
`UnscopedNotice` pra avisar honestamente que 5 widgets (`summary`,
`queue-daily`, `outcomes-donut`, `placement-distribution`,
`mission-distribution`) não respeitavam o filtro de modo (item D,
documentado como pendente na época). A spec 014 do analytics-service fecha
essa lacuna — os 5 widgets passam a vir corretamente filtrados do backend.
O aviso fica obsoleto: sem isso ser removido, o código morto confundiria
qualquer leitura futura (por que existe uma checagem que nunca é
verdadeira?).

## Comportamento

- `UNSCOPED_WIDGETS`, `showsUnscopedNotice`, `UnscopedNotice` e as 4
  chamadas (`<UnscopedNotice show={...} />` nos 4 widgets) SHALL ser
  removidos — nenhum comportamento visível novo, só a interface fica
  limpa (os widgets já mostravam o número certo desde que o backend
  passou a filtrar; só a legenda de aviso deixa de aparecer, porque deixou
  de ser verdade).

## Critérios de Aceite

1. Filtrar por um modo específico -> `summary`/`queue-daily`/
   `outcomes-donut`/`placement-distribution`/`mission-distribution` SEM
   nenhuma legenda de aviso, números corretos (mesmo dado real usado pra
   validar a spec 014).
2. `npx tsc --noEmit` limpo, sem referências soltas aos símbolos removidos.

## Fora de escopo

- Qualquer mudança visual além da remoção do aviso.
