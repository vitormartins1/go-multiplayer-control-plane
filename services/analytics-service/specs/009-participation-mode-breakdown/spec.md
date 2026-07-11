# 009 — Quebra por modo nas participações (host_world_dropin)

## Contexto

Revisão da aba Analytics do jogo (admin-ui, 2026-07-11) com Capoeira Souls e
Neon Breach já com dado real: um jogo com vários modos host_world_dropin
(Capoeira Souls tem 6: Roda Aberta, Jogo de Angola, Batizado, Desafio do
Mestre, Roda Fechada, Maculelê, Roda Regional) mostra "Participações por
papel" como um bolo único — sem distinguir qual modo gerou qual evento.

O `mode_id` já é publicado pelo session-service no envelope
`participation_event` (SNS, `sns_publisher.go`, ADR-0024, campo
`ModeID string \`json:"modeId,omitempty"\``) — a lacuna é só no
analytics-service, que descarta esse campo na desserialização
(`consumer.go:parseParticipation`) e nunca chegou a existir na tabela de
rollup (`game_participation_rollup`, chave hoje `(game_id, role, event,
outcome)`).

**Escopo desta fatia:** só a participação (host_world_dropin) — é o gap
concretamente observado. O mesmo `mode_id` também já é publicado nos
envelopes de `win_loss_draw` (queue_arena), `placement` (battle_royale) e
`collective` (coop_mission), e as 3 tabelas de rollup correspondentes
(`game_rollup`, `game_placement_rollup`, `game_mission_rollup`) têm a MESMA
lacuna — hoje são `PRIMARY KEY (game_id)`, uma linha só por jogo, ainda mais
grosseiro que a participação. É uma fatia estruturalmente idêntica a esta,
mas maior (muda a chave primária das 3 tabelas, não só adiciona dimensão a
uma que já é composta) — fica de fora deliberadamente, para não inflar esta
entrega; pode virar spec própria depois se fizer sentido.

## Comportamento

- `game_participation_rollup` ganha a coluna `mode_id` (dado antigo, sem essa
  dimensão, cai no bucket `''` — "sem modo" — mesmo padrão já usado pro
  `outcome` na spec 007). Chave primária passa a
  `(game_id, mode_id, role, event, outcome)`.
- O consumer do analytics-service extrai `modeId` do envelope
  `participation_event` (hoje descartado) e persiste no rollup.
- `GET /admin/games/{gameId}/analytics` (analytics-service) devolve `modeId`
  em cada entrada de `roleParticipations`.
- admin-bff repassa o campo (passthrough, mesmo padrão do resto do DTO).
- admin-ui:
  - `ParticipationStat` ganha `modeId: string` (`""` = eventos de antes desta
    mudança, sem modo).
  - A seção "Participações por papel" ganha um seletor de modo (só modos
    host_world_dropin do jogo, mais a opção "Todos os modos", que é o
    default) — filtra o gráfico E a tabela pelo modo escolhido.
  - Nome de exibição do modo vem de `game.multiplayerModes` (já carregado
    pela página); `modeId` sem modo correspondente (ou `""`) mostra "sem
    modo" no seletor/tabela.

## Critérios de aceite

- [ ] Reportar um evento de participação num mundo aberto com um modo
      nomeado (`OpenWorld` com `modeId`) → o rollup grava a linha com esse
      `mode_id`; consultar `GET /admin/games/{id}/analytics` devolve o campo.
- [ ] Dois modos diferentes do mesmo jogo (ex.: "Roda Aberta" e "Batizado")
      geram linhas SEPARADAS no rollup — a agregação de um não soma no
      outro.
- [ ] Dado gravado antes desta migration continua aparecendo (bucket
      `mode_id=''`), sem quebrar a leitura nem duplicar contagem.
- [ ] admin-ui: selecionar um modo no seletor filtra o gráfico "Participações
      por papel" e a tabela abaixo; "Todos os modos" mostra o agregado atual
      (comportamento de hoje, inalterado).
- [ ] Reentrega do mesmo evento (at-least-once do SQS) continua idempotente
      (idempotência de `processed_participations` por `event_id` não muda).
