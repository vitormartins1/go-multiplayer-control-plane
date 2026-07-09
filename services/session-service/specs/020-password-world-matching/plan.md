# Plan: 020-password-world-matching

**Serviço:** session-service · **Spec:** `spec.md` (Proposta 2026-07-09) · **ADR:** 0023 (follow-up #1, lado session)

Como o session passa a (a) receber e hashear a senha do host no `OpenWorld` e (b) expor o hash no diretório de invadíveis. Fatia **aditiva** sobre H0 (spec 011) e H1a (spec 012).

## Decisões técnicas

### D1 — senha entra em `WorldAttributes`, não como parâmetro posicional novo

`WorldAttributes` (spec 012: `{SoulLevel, WeaponTier int}`) ganha `Password string`. Mesma razão de D1 da spec 012: evitar crescer a assinatura de `OpenWorld` a cada atributo novo. O campo é **plaintext na entrada** — o `Service.OpenWorld` hasheia antes de gravar em `Session`; `WorldAttributes.Password` não sobrevive além da chamada.

### D2 — hash: SHA-256 hex, função pura em `domain`, sem pacote compartilhado

```go
// domain/password.go
func HashWorldPassword(plaintext string) string {
    if plaintext == "" {
        return ""
    }
    sum := sha256.Sum256([]byte(plaintext))
    return hex.EncodeToString(sum[:])
}
```

Vazio → vazio (nunca o hash de `""`), para um mundo sem senha nunca "bater" com um invasor que também não informou senha (spec critério de aceite 3). **Decisão deliberada de duplicar** essa função em `matchmaking-service` (`matchmaking/011`) em vez de um módulo Go compartilhado: os dois serviços já são independentes (ADR-0001, monorepo com módulos independentes) e a função é trivial (5 linhas) — introduzir uma dependência cruzada só para isto não compensa. Risco anotado: se o algoritmo mudar, os dois lugares precisam mudar juntos (documentado no código dos dois lados, comentário cruzado).

### D3 — `Session.PasswordHash` e `InvadableWorld.PasswordHash`

- `Session` += `PasswordHash string`.
- `InvadableWorld` (spec 012) += `PasswordHash string`.
- `Service.OpenWorld`: `sess.PasswordHash = domain.HashWorldPassword(attrs.Password)`.
- `Service.ListInvadableWorlds`: inclui `PasswordHash: sess.PasswordHash` no `InvadableWorld` montado (mesma varredura sob lock da spec 012, D2 — só mais um campo copiado).

### D4 — proto aditivo + `buf`

- `OpenWorldRequest` += `string password = 6;` (campo 6 — os índices 1–5 já estão ocupados por `host_player_id`/`game_id`/`host_soul_level`/`weapon_tier`/`mode_id`, spec 011/012/017; conferir o índice livre real no `.proto` atual antes de gerar).
- `InvadableWorld` += `string password_hash = 5;` (índices 1–4 já ocupados: `session_id`/`host_soul_level`/`weapon_tier`/`free_invader_slots`, spec 012).
- `PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço (nunca `sed`).

### D5 — transport

- Handler `OpenWorld`: `attrs.Password = req.GetPassword()` antes de chamar o service.
- Handler `ListInvadableWorlds`: `pb.InvadableWorld{..., PasswordHash: w.PasswordHash}`.

## Domínio (`internal/domain/session.go` + novo `password.go`)

- `WorldAttributes` += `Password string`.
- `Session` += `PasswordHash string`.
- `InvadableWorld` += `PasswordHash string`.
- Novo `HashWorldPassword(plaintext string) string` (D2).

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/session.go` | `WorldAttributes.Password`, `Session.PasswordHash`, `InvadableWorld.PasswordHash` |
| `internal/domain/password.go` (novo) | `HashWorldPassword` |
| `internal/domain/password_test.go` (novo) | tabela: vazio→vazio, senhas iguais→hash igual, diferentes→hash diferente |
| `internal/service/session_service.go` | `OpenWorld` hasheia e grava; `ListInvadableWorlds` copia o hash |
| `internal/service/host_world_test.go` | testes de senha (guarda hash; sem senha = hash vazio; diretório reflete) |
| `proto/session/v1/session.proto` | campos 6/5 (D4); `buf generate` |
| `internal/transport/grpc_server.go` | `OpenWorld`/`ListInvadableWorlds` passam o campo novo |
| `internal/transport/host_world_grpc_test.go` | e2e bufconn: OpenWorld com senha → hash aparece no diretório |

`cmd/server/main.go` — sem mudança.

## Testes

- Baseline `go test ./services/session-service/... -race` verde antes.
- `domain`: `HashWorldPassword` — vazio→vazio; determinístico (mesma entrada → mesmo hash); duas senhas diferentes → hashes diferentes (sem colisão nos casos de teste).
- `service`: `OpenWorld` com senha grava `PasswordHash`; sem senha, `PasswordHash` vazio; `ListInvadableWorlds` reflete o hash de cada mundo.
- `transport` (bufconn): fluxo completo OpenWorld(senha) → ListInvadableWorlds mostra o hash certo.
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: `HashWorldPassword` + campos (`WorldAttributes`, `Session`, `InvadableWorld`) + testes de domínio.
3. Service: `OpenWorld` hasheia e grava; `ListInvadableWorlds` copia o hash; testes.
4. Proto (campos 6/5) + `buf generate` + transport (handlers) + teste bufconn.
5. `go test ./... -race` verde (baseline + novo); lint.

## Riscos / gotchas

- **Índices de campo no proto:** conferir os índices realmente livres em `OpenWorldRequest`/`InvadableWorld` no `.proto` atual antes de gerar — a spec lista 6/5 com base no estado de 2026-07-09, mas outra fatia pode ter ocupado um índice nesse meio tempo.
- **Hash duplicado em dois serviços (D2):** documentar com comentário cruzado nos dois arquivos (`session-service/internal/domain/password.go` ↔ `matchmaking-service`) apontando um pro outro, para quem for mudar o algoritmo lembrar dos dois lados.
- Sem migration (sessão em memória).
