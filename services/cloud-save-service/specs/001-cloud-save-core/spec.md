# Spec: 001-cloud-save-core

**Serviço:** cloud-save-service
**Status:** Approved
**Data:** 2026-06-28

## Contexto

Núcleo do cloud-save-service (≈ GameKit Game State Saving): guarda o **arquivo de save** do jogador — um **blob opaco** (estado serializado do jogo), distinto do player-data (dados pequenos e estruturados). A persistência server-side dá save na nuvem e cross-device. É o primeiro serviço a usar **object storage** (MinIO/S3) em vez de Postgres — estreia o polyglot do ADR-0015. Valida o JWT do identity via `shared/auth`.

Decisões de escopo (ver "Fora do escopo"): **um save por jogador** (múltiplos slots é "próximo"); **last-write-wins** (versionamento e resolução de conflito são "próximo"); o conteúdo do save é **opaco** ao serviço.

## Comportamento

### Autenticação (todos os endpoints)
- THE SYSTEM SHALL exigir `Authorization: Bearer <jwt>` válido (JWKS do identity); senão -> 401 **ErrUnauthorized**.
- THE SYSTEM SHALL associar o save ao jogador do `sub` do token, nunca a um id do path/corpo.

### Salvar
- WHEN PUT /me/save com o blob no corpo -> grava (cria ou sobrescreve) o save do jogador e retorna 204.
- THE SYSTEM SHALL preservar o `Content-Type` da requisição (default `application/octet-stream`) e devolvê-lo no carregamento.
- IF o corpo exceder o limite de tamanho -> 413 **ErrTooLarge**.
- THE SYSTEM SHALL sobrescrever o save anterior (**last-write-wins** no MVP).

### Carregar
- WHEN GET /me/save -> 200 com o blob salvo, no `Content-Type` em que foi gravado.
- IF não existir save para o jogador -> 404 **ErrNotFound**.

## Contratos de Interface

### CloudSaveService (Port de entrada)
- `Save(ctx, playerID string, snapshot Snapshot) error`
- `Load(ctx, playerID string) (Snapshot, error)`

### SaveStore (Port de saída)
- `Put(ctx, playerID string, snapshot Snapshot) error` — sobrescreve
- `Get(ctx, playerID string) (Snapshot, error)` — `ErrNotFound` se ausente

### TokenVerifier (Port de saída, usada no transport)
- `Verify(ctx, token) (playerID string, error)` — `shared/auth` (JWKS do identity).

### Tipo
- `Snapshot{ Blob []byte; ContentType string }` — blob opaco + content-type.

## Modelo de Armazenamento

**Object storage (MinIO/S3, ADR-0015)**

| Item | Valor |
|---|---|
| Bucket | `cloud-save` |
| Object key | o `player_id` (`sub` do JWT; aceita `guest-…`) — **um objeto por jogador** |
| Body | o blob de save |
| Metadata | `Content-Type` (e o `LastModified` do próprio storage) |

## Regras de Validação

| Item | Regra | Erro |
|---|---|---|
| Token | válido (JWKS), não expirado | ErrUnauthorized |
| Tamanho do corpo | ≤ limite configurado (ex. 5 MiB) | ErrTooLarge (413) |
| Conteúdo | **não** validado (opaco) | — |

## Casos de Teste Obrigatórios

- [ ] PUT grava um blob; GET retorna exatamente os mesmos bytes e o mesmo Content-Type
- [ ] PUT sobrescreve o save anterior (last-write-wins)
- [ ] GET sem save existente -> 404 ErrNotFound
- [ ] corpo acima do limite -> 413 ErrTooLarge
- [ ] requisição sem Bearer / token inválido -> 401
- [ ] saves isolados entre jogadores (o `sub` do token determina o dono)

## Fora do escopo

- **Versionamento de save** (número/histórico) — próxima feature
- **Resolução de conflito** (cliente vs nuvem) — próxima feature
- **Múltiplos slots por jogador** — próxima feature (MVP é um save por jogador)
- **Compressão / sync delta** — backlog
- **Quota de armazenamento por jogador** — backlog
