# Spec: 003-admin-list-users

**Serviço:** identity-service
**Status:** Done
**Data:** 2026-06-29

## Contexto

Iteração 2 do painel admin (ADR-0016): a página **User base** precisa **listar e contar** os usuários cadastrados — dado que o identity ainda não expõe (só `Create`/`FindByHandle`, features 001/002). Esta feature adiciona um endpoint **admin read-only** que lista as contas (paginadas) e devolve o total.

É **control plane interno** — **sem auth** no MVP (consistente com `achievements/003-admin-read` e a postura do `admin-bff`; auth real é evolução). É a fonte de dados; o `admin-bff` e o `admin-ui` que a consomem são features à parte (próximos passos).

## Comportamento

### Listar usuários (admin)
- WHEN `GET /admin/players?limit=&offset=` -> retorna a página de contas ordenadas por `created_at` **desc** (mais recentes primeiro) e o **total** de contas.
- THE SYSTEM SHALL retornar de cada conta apenas `id`, `handle` e `createdAt` — **nunca** o `password_hash` (nem qualquer credencial).
- THE SYSTEM SHALL aplicar paginação: `limit` default **50**, máximo **200**; `offset` default **0**. Valores inválidos (não numérico, negativo) -> usa o default; `limit` acima do máximo -> satura no máximo.
- THE SYSTEM SHALL devolver `total` = contagem de todas as contas (independente da página), para a página exibir "N usuários".
- WHEN não houver contas (ou a página passar do fim) -> `players` vazio e `total` coerente (não é erro).

### Invariante
- THE SYSTEM SHALL manter o endpoint `/admin/...` **sem auth** no MVP (uso interno/local; a barreira é a rede).

## Contratos de Interface

### API HTTP (nova rota)
| Método | Rota | Retorno |
|---|---|---|
| GET | `/admin/players?limit=&offset=` | `{ "total": <int>, "players": [ { "id", "handle", "createdAt" } ] }` |

### AccountRepository (Port de saída — novos métodos)
- `ListAccounts(ctx, limit, offset int) ([]Account, error)` — contas por `created_at` desc
- `CountAccounts(ctx) (int, error)` — total de contas

### AdminService (Port de entrada — nova, ou método no service existente)
- `ListPlayers(ctx, limit, offset int) (players []Account, total int, error)`

## Modelo de Dados

Sem mudança de schema — usa a tabela `accounts` existente (`id, handle, password_hash, created_at`). Só leitura; o `password_hash` é lido apenas pelo fluxo de auth, **não** pelo endpoint admin (que projeta `id, handle, created_at`).

## Regras de Validação
| Campo | Regra | Comportamento |
|---|---|---|
| `limit` | inteiro 1..200 | inválido/ausente -> 50; > 200 -> 200 |
| `offset` | inteiro >= 0 | inválido/ausente/negativo -> 0 |

## Casos de Teste Obrigatórios

- [ ] `GET /admin/players` -> lista contas (id/handle/createdAt) ordenadas por createdAt desc + `total`
- [ ] A resposta **nunca** inclui `password_hash` / `passwordHash`
- [ ] `limit`/`offset` paginam corretamente; `total` é o total global (não o da página)
- [ ] `limit` acima de 200 satura em 200; valores inválidos -> defaults
- [ ] Sem contas -> `players` vazio, `total` 0 (não erro)
- [ ] O endpoint admin **não** exige `Authorization`

## Fora do escopo

- **Busca/filtro por handle** e **ordenação configurável** — incremental (2.1)
- **Detalhe de um usuário** (`GET /admin/players/{id}`) — adicionar se o painel pedir
- **Auth** nos endpoints admin — interno/local no MVP; provável ADR futuro
- **Ações** (banir, remover, editar) — read-only
- **Consumo no `admin-bff` / `admin-ui`** — features à parte (próximos passos)
- **Cursor-based pagination** — offset/limit basta no MVP
