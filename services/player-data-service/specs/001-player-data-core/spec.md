# Spec: 001-player-data-core

**Serviço:** player-data-service
**Status:** Approved
**Data:** 2026-06-28

## Contexto

Núcleo do player-data-service (≈ GameKit User Gameplay Data): armazena **dados de gameplay por jogador** em pares chave-valor organizados por **namespace** (ex. `inventory`, `stats`, `settings`). A persistência server-side dá **cross-play automático** (o mesmo jogador vê seus dados em qualquer dispositivo). É o segundo serviço a **validar o JWT do identity** (ADR-0009) — e o primeiro a consumir o verificador compartilhado `shared/auth`.

Decisões de escopo (ver "Fora do escopo"): valor é **JSON opaco** (JSONB no banco); escrita é **last-write-wins** (sem optimistic locking, que é a próxima feature).

## Comportamento

### Autenticação (todos os endpoints)
- THE SYSTEM SHALL exigir `Authorization: Bearer <jwt>` válido (assinatura via JWKS do identity, não expirado); senão -> 401 **ErrUnauthorized**.
- THE SYSTEM SHALL identificar o jogador pelo `sub` do token; o dado é sempre do jogador autenticado, nunca de um id vindo do path/corpo.

### Gravar um valor
- WHEN PUT /me/data/{namespace}/{key} com um JSON válido no corpo -> grava (cria ou sobrescreve) e retorna 204.
- IF `namespace` ou `key` violarem as regras de validação -> 400 **ErrInvalidKey**.
- IF o corpo não for JSON válido -> 400 **ErrInvalidValue**.
- THE SYSTEM SHALL sobrescrever o valor existente (**last-write-wins** no MVP) e registrar o momento da escrita.

### Ler um valor
- WHEN GET /me/data/{namespace}/{key} -> 200 com o valor (o JSON gravado).
- IF a chave não existir para o jogador -> 404 **ErrNotFound**.

### Ler um namespace inteiro
- WHEN GET /me/data/{namespace} -> 200 com um objeto JSON `{ "<key>": <value>, ... }` de todos os pares do jogador naquele namespace (objeto vazio se nenhum).

### Remover um valor
- WHEN DELETE /me/data/{namespace}/{key} -> 204.
- THE SYSTEM SHALL ser idempotente: remover uma chave inexistente também retorna 204.

## Contratos de Interface

### PlayerDataService (Port de entrada)
- `Get(ctx, playerID, namespace, key) (value []byte, error)`
- `GetNamespace(ctx, playerID, namespace) (map[string][]byte, error)`
- `Put(ctx, playerID, namespace, key string, value []byte) error`
- `Delete(ctx, playerID, namespace, key string) error`

### PlayerDataRepository (Port de saída)
- `Get`, `GetNamespace`, `Put` (upsert), `Delete` — mesma forma; `value` é JSON opaco (`[]byte`).

### TokenVerifier (Port de saída, usada no transport)
- `Verify(ctx, token) (playerID string, error)` — implementada por `shared/auth` (JWKS do identity).

## Modelo de Dados

**player_data**

| Campo | Tipo | Restrições |
|---|---|---|
| player_id | TEXT | id do jogador (`sub` do JWT; aceita `guest-…`) |
| namespace | TEXT | agrupador, ex. `inventory` |
| key | TEXT | chave dentro do namespace |
| value | JSONB | valor JSON opaco |
| updated_at | TIMESTAMPTZ | momento da última escrita |
|  | **PK (player_id, namespace, key)** | unicidade por jogador/namespace/chave |

## Regras de Validação

| Item | Regra | Erro |
|---|---|---|
| Token | válido (JWKS), não expirado | ErrUnauthorized |
| namespace | obrigatório; 1–64 chars; `[a-zA-Z0-9_-]` | ErrInvalidKey |
| key | obrigatório; 1–128 chars; `[a-zA-Z0-9_.-]` | ErrInvalidKey |
| value | JSON sintaticamente válido | ErrInvalidValue |

## Casos de Teste Obrigatórios

- [ ] PUT grava um valor; GET retorna exatamente o mesmo JSON
- [ ] PUT na mesma chave sobrescreve (last-write-wins)
- [ ] GET de chave inexistente -> 404 ErrNotFound
- [ ] GET de namespace retorna todos os pares do jogador (e objeto vazio quando nenhum)
- [ ] DELETE remove; GET seguinte -> 404; DELETE de inexistente -> 204 (idempotente)
- [ ] namespace ou key inválido -> 400 ErrInvalidKey
- [ ] corpo não-JSON no PUT -> 400 ErrInvalidValue
- [ ] requisição sem Bearer / token inválido -> 401
- [ ] dados isolados entre jogadores (o `sub` do token determina o dono)

## Fora do escopo

- **Optimistic locking / versionamento** (evitar sobrescrita concorrente) — próxima feature (FEATURE-MAP "próximo")
- **Quota / limite de tamanho** por jogador — backlog
- **Operações em lote** (bulk get/set) — backlog
- **TTL / expiração** de dados
- **Schema/validação do conteúdo** do value (é opaco para o serviço)
