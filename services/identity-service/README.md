# identity-service

≈ **GameKit Identity & Auth** (Cognito). É a **fundação de auth** do backend (ADR-0009): registra contas, emite o **session JWT (Ed25519)** e publica as chaves públicas (JWKS). Todos os outros serviços validam o token emitido aqui.

- **Protocolo:** REST (`net/http`, Go 1.22) · **Porta host:** 8081
- **Storage:** PostgreSQL (`identity-postgres`, DB `identity`)
- **Dependências:** nenhuma (é a base) · **Depende de quem?** todos dependem dele

## API

| Método | Rota | Descrição |
|---|---|---|
| POST | `/accounts` | registrar conta (`handle`, `password`) |
| POST | `/sessions` | login → JWT |
| POST | `/sessions/anonymous` | login guest (token `anon`) |
| POST | `/sessions/steam` | login Steam: `{ticket, gameId?}` → JWT (claim `idp: steam`); cria a conta na primeira vez (spec 005). 401 `invalid_ticket`, 403 `banned`, 503 `provider_unavailable`, 404 se não configurado |
| GET | `/.well-known/jwks.json` | chaves públicas para validar o JWT offline |

## Modelo de dados

**`accounts`** (PostgreSQL)

| Coluna | Tipo | Notas |
|---|---|---|
| id | UUID | PK |
| handle | TEXT | único (case-insensitive); 3–20 chars |
| password_hash | TEXT | hash bcrypt; nunca em claro |
| created_at | TIMESTAMPTZ | default `now()` |

**`external_identities`** (spec 005): `(provider, external_id)` PK → `account_id` (FK `accounts`). Conta criada
pelo login Steam tem `password_hash` vazio — o login por senha nunca autentica nela.

> Login anônimo (`/sessions/anonymous`) não persiste nada — o token guest é efêmero.

### Login Steam (spec 005)

O cliente pede um ticket com `ISteamUser::GetAuthTicketForWebApi("<STEAM_TICKET_IDENTITY>")`, espera o callback
`GetTicketForWebApiResponse_t` e manda o ticket em hex para `POST /sessions/steam`. O identity valida na Web API da
Steam (`ISteamUserAuth/AuthenticateUserTicket`, publisher key), acha ou cria a conta vinculada ao SteamID64
(`external_identities`) e devolve o JWT de sempre. Env: `STEAM_WEB_API_KEY`, `STEAM_APP_ID`,
`STEAM_TICKET_IDENTITY` (padrão `control-plane`), `STEAM_API_BASE_URL` (padrão `https://partner.steam-api.com`).

## Observabilidade

`/metrics` (RED + runtime) na porta 8081; aparece no dashboard **Service Overview** (`$job=identity-service`). Tracing no Tempo. Banco no dashboard **PostgreSQL** (`identity-postgres`).

## Testar / Specs

- Manual: [`api/identity.http`](api/identity.http) (GoLand ▶ / VS Code REST Client)
- Specs: [`specs/`](specs/) — `001-auth-core`, `002-anonymous-login`
