# Spec 008 — Expor outcome no DTO + corrigir mount da migration 006

> Status: **implementada** (achado durante a Fase 1 do dashboard de Analytics
> por jogo, 2026-07-10)
> Fatia: pequena/aditiva — fecha um follow-up já registrado como "fora de
> escopo" na spec 007, mais um bug de infra encontrado no processo.

## Contexto

A spec 007 (`participation-event-id-and-outcome`, Done 2026-07-09) já tinha
implementado `outcome` como parte da chave de `game_participation_rollup` e
da leitura (`SELECT role, event, outcome, count ...`), mas deixou
explicitamente "fora de escopo": *"UI do funil (H3) consumindo a nova
dimensão — fica pro admin-bff/admin-ui"*. O DTO HTTP (`participationStatDTO`)
nunca ganhou o campo `Outcome` — o valor era lido do banco e descartado na
serialização.

**Achado à parte, mais sério:** testando o endpoint pra validar o gráfico
novo (spec admin-ui/054), `GET /admin/games/{id}/analytics` devolvia `500`
pra QUALQUER jogo. Causa: a migration 006 (que criou a própria coluna
`outcome` da spec 007) nunca foi adicionada ao mount do
`docker-compose.yml` — a query `SELECT ... outcome ...` já existente no
código referenciava uma coluna inexistente em qualquer ambiente local. Não
foi introduzido por este trabalho; só ficou exposto ao testar o endpoint de
detalhe por jogo, algo que aparentemente não tinha sido exercitado desde a
007.

## Comportamento (EARS)

1. `participationStatDTO` (transport) DEVE incluir `outcome` — o valor já
   lido pelo repositório, só não estava sendo serializado.
2. O `docker-compose.yml` DEVE montar a migration 006 no `analytics-postgres`
   (estava faltando desde a spec 007).
3. Nenhuma mudança de comportamento além de expor o campo — a lógica de
   agregação (spec 007) já estava correta.

## Critérios de aceite

- [x] `GET /admin/games/{id}/analytics` responde `200` (não mais `500`) —
      confirmado com dados reais (Packet Arena, Neon Breach).
- [x] `roleParticipations[].outcome` presente e correto na resposta.
- [x] admin-bff (`domain.ParticipationStat`) espelha o campo — passthrough
      via JSON, sem tradução manual.
- [x] Migration 006 aplicada manualmente no volume em uso (initdb não
      re-roda) + mount adicionado ao compose pra ambientes novos.
- [x] `-race` verde; teste de regressão cobrindo `outcome` no DTO.

## Fora de escopo

- Exibir `outcome` na UI além da tabela/legenda já existente (spec 054 já
  cobre isso — badge "Sucesso"/"Falha" na tabela de participações).
