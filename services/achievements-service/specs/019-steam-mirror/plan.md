# Plan: 019-steam-mirror

| Tema | Decisão | Por quê |
|---|---|---|
| Onde fica o pendente | a própria linha de `player_achievements` (colunas `steam_sync_state`, `steam_sync_attempts`, `steam_next_attempt_at`, `steam_sync_error`, `steam_synced_at`); pendente = desbloqueada + conquista com `steam_api_name` + estado vazio | os três caminhos de desbloqueio (HTTP, match, participação) não mudam; o backfill (item 5) sai de graça; sem tabela de outbox paralela para manter coerente |
| Quem sincroniza | um worker no binário **consumer** (goroutine com context + WaitGroup, laço a cada 15 s, lote de 50) | o consumer já é o processo de fundo do serviço; o server HTTP segue só servindo |
| Concorrência entre réplicas | `SELECT … FOR UPDATE SKIP LOCKED` numa transação curta que já empurra `steam_next_attempt_at` (lease de 2 min) | duas réplicas do consumer não mandam o mesmo desbloqueio ao mesmo tempo |
| SteamID | identity `GET /admin/players/{id}/external-identities` (internal key) | o vínculo mora no identity (ADR-0031); o achievements não guarda SteamID |
| Steam | `POST {base}/ISteamUserStats/SetUserStatsForGame/v1/` form `key, steamid, appid, count=1, name[0], value[0]=1`; sucesso = `response.result == 1` | Web API de publisher; mesma base/chave do login (`partner.steam-api.com`) |
| Backoff | `1 min × 2^(tentativa-1)`, teto 1 h; `failed` após 10 | Steam fora do ar por horas não vira martelada |

Arquivos: `migrations/011_steam_mirror.sql`; `internal/domain/{achievement.go,steam.go}`;
`internal/repository/{postgres_achievement.go,postgres_steam_sync.go}`; `internal/steam/client.go`;
`internal/identityclient/client.go`; `internal/service/steam_mirror.go`; `cmd/consumer/main.go`; transport (campo
`steamApiName`); identity `006`; admin-bff/admin-ui (campo + selo); compose (envs).
