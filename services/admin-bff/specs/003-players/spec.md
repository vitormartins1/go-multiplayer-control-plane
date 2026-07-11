# Spec: 003-players

**Serviço:** admin-bff
**Status:** Done
**Data:** 2026-06-29

## Contexto

Iteração 2 do painel (ADR-0016): a página **User base** precisa listar os usuários. O `identity/003-admin-list-users` já expõe `GET /admin/players` (lista paginada + total, sem auth interno). Esta feature adiciona ao `admin-bff` o endpoint que o `admin-ui` consome — agregando/repassando a lista do identity, no mesmo padrão das demais (adapter + Port).

Read-only, sem auth no MVP. Lê **só a API** do identity (nunca o banco — ADR-0016).

## Comportamento

### Listar usuários
- WHEN `GET /api/players?limit=&offset=` -> chama o `GET /admin/players` do identity e devolve `{ total, players: [{id, handle, createdAt}] }`.
- THE SYSTEM SHALL repassar `limit`/`offset` ao identity (a política de paginação/clamp é do identity).
- IF o identity estiver indisponível -> `502` (**players_unavailable**), como o padrão de fonte indisponível.

## Contratos de Interface

### API HTTP (nova rota)
| Método | Rota | Retorno |
|---|---|---|
| GET | `/api/players?limit=&offset=` | `{ total, players: [{id, handle, createdAt}] }` |

### PlayerDirectory (Port de saída — nova)
- `ListPlayers(ctx, limit, offset int) (PlayerPage, error)` — lê a API admin do identity; `ErrIdentityUnavailable` se inacessível.

### Tipos (domínio)
- `Player{ ID, Handle string; CreatedAt time.Time }`
- `PlayerPage{ Total int; Players []Player }`

## Casos de Teste Obrigatórios

- [ ] `GET /api/players` -> repassa a página do identity (total + players) (identity fake)
- [ ] `limit`/`offset` são repassados ao identity
- [ ] identity indisponível -> `502`
- [ ] o adapter mapeia o JSON do identity (`{total, players[]}`) para `PlayerPage`

## Fora do escopo

- **Paginação/clamp** — é responsabilidade do identity (o BFF só repassa)
- **Busca/filtro, detalhe de usuário** — incremental
- **Auth** — interno/local no MVP
- **Consumo no `admin-ui`** — feature à parte (próximo passo)
