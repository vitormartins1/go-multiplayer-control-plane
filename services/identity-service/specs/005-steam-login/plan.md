# Plan: 005-steam-login

- **D1 — Ports novas no domain:** `ExternalIdentityVerifier` (`Provider()`, `Verify(ctx, ticket)`) e
  `ExternalIdentityRepository` (`FindByExternal`, `CreateWithExternal`). O `AuthService` ganha
  `AuthenticateExternal(ctx, provider, ticket, gameID)`; verificadores entram por option no `service.New`.
- **D2 — Conta sem senha:** `password_hash = ''` — o bcrypt nunca aceita, então o login por senha fica impossível
  sem coluna nova.
- **D3 — Handle determinístico:** `steam_` + 14 últimos dígitos do SteamID64 (cabe no `^[a-zA-Z0-9_]{3,20}$`; o
  SteamID64 é base fixa + accountID de 32 bits, então os 14 últimos dígitos não colidem entre contas Steam). Se o
  handle já existir (alguém registrou à mão), uma tentativa com sufixo aleatório.
- **D4 — Vínculo na mesma transação da conta** (`external_identities` com PK `(provider, external_id)`); violação
  do PK = corrida com outro login do mesmo jogador → relê.
- **D5 — Adapter** `internal/steam` com `http.Client` de 5s; ticket validado como hex (≤ 4096 chars) antes da rede.
- Arquivos: `migrations/002_external_identities.sql`, `internal/domain/account.go`/`errors.go`,
  `internal/service/federated.go`, `internal/repository/postgres_account.go`, `internal/steam/verifier.go`,
  `internal/transport/http_handler.go`, `cmd/server/main.go`, README e compose (envs vazias).
