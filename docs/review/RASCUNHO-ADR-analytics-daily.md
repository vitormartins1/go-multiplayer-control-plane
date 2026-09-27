# Rascunho de ADR — uma tabela diária só no analytics (para aprovação; não gravado em `docs/adr/`)

> Pendência da revisão 2026-09 ("consolidar as `*_daily`"). Precisa do ok do dono porque altera o modelo de dados
> da ADR-0021. Se aprovado, vira `docs/adr/0033-analytics-activity-daily.md` + spec `analytics/017`.

## Contexto

O analytics tem cinco tabelas diárias com o **mesmo formato** — `(game_id, [mode_id], day) → contador` — uma por
arquétipo/fonte, criadas em migrations diferentes:

| Tabela | Contador | Fonte | `mode_id` |
|---|---|---|---|
| `game_daily` | `matches` | resultado `win_loss_draw` (queue_arena) | não tem |
| `game_placement_daily` | `matches` | `placement` (battle_royale) | tem |
| `game_mission_daily` | `matches` | `collective` (coop_mission) | tem |
| `game_participation_daily` | `events` | `participation_event` (host_world_dropin) | tem |
| `game_session_daily` | `count` | `session_closed` (todos) | tem |

Custos de hoje: cinco `INSERT … ON CONFLICT` quase iguais no repositório, cinco `SELECT` na série diária, a
`game_daily` sem modo (o filtro por modo não funciona para queue_arena), e cada fonte nova pede tabela + migration +
query.

## Alternativas

- **A — manter como está.** Zero trabalho; o custo acima continua e cresce a cada fonte.
- **B (recomendada) — `game_activity_daily(game_id, mode_id, day, kind, count)`**, com `kind` em
  `match | placement | mission | participation | session_closed`. Um upsert e uma query por série (`WHERE kind = …`
  ou `GROUP BY kind`); `queue_arena` ganha modo. Migration copia os dados das cinco (a `game_daily` antiga entra com
  `mode_id = ''`, como os demais legados) e as antigas saem numa migration seguinte, depois de um deploy com as duas.
- **C — views sobre as tabelas atuais.** Unifica a leitura sem migrar dado, mas a escrita continua em cinco lugares.

## Consequências de B

- **Positivas:** menos código no repositório do analytics; fonte nova = um valor de `kind`; filtro por modo em todos
  os arquétipos.
- **Negativas:** migration de dados (pequena no volume atual) e duas etapas de deploy; os testes de integração do
  analytics mudam de tabela.
- **Contrato:** nenhum — a API do analytics e o BFF não mudam.
