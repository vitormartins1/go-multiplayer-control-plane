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
| GET | `/.well-known/jwks.json` | chaves públicas para validar o JWT offline |

## Modelo de dados

**`accounts`** (PostgreSQL)

| Coluna | Tipo | Notas |
|---|---|---|
| id | UUID | PK |
| handle | TEXT | único (case-insensitive); 3–20 chars |
| password_hash | TEXT | hash bcrypt; nunca em claro |
| created_at | TIMESTAMPTZ | default `now()` |

> Login anônimo (`/sessions/anonymous`) não persiste nada — o token guest é efêmero.

## Observabilidade

`/metrics` (RED + runtime) na porta 8081; aparece no dashboard **Service Overview** (`$job=identity-service`). Tracing no Tempo. Banco no dashboard **PostgreSQL** (`identity-postgres`).

## Testar / Specs

- Manual: [`api/identity.http`](api/identity.http) (GoLand ▶ / VS Code REST Client)
- Specs: [`specs/`](specs/) — `001-auth-core`, `002-anonymous-login`
