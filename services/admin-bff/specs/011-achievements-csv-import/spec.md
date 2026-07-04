# Spec 011 — Importação em lote de conquistas via CSV (admin-bff)

## Contexto

O admin-ui permite cadastrar conquistas individualmente. Para jogos com muitos achievements, o cadastro manual é lento. Queremos um endpoint de importação em lote que receba um array de conquistas JSON (parsado no frontend a partir de um CSV) e retorne o resultado por linha.

O achievements-service já tem `POST /admin/games/{gameId}/achievements` para criação unitária — esta spec apenas adiciona a camada de orquestração no BFF.

## Endpoint

```
POST /api/games/{gameId}/achievements/import
Content-Type: application/json
```

### Corpo da requisição

```json
{
  "achievements": [
    { "id": "first_win", "name": "First Win", "description": "Win your first match", "eventType": "match_won", "target": 1 },
    { "id": "ten_wins",  "name": "Ten Wins",  "description": "Win ten matches",       "eventType": "match_won", "target": 10 }
  ]
}
```

Campos por item: idênticos ao `POST /api/games/{gameId}/achievements` existente.

### Corpo de resposta (200)

```json
{
  "created": 1,
  "failed": 1,
  "results": [
    { "row": 1, "id": "first_win", "status": "created" },
    { "row": 2, "id": "ten_wins",  "status": "duplicate", "error": "achievement_exists" }
  ]
}
```

**Sempre 200** — a resposta HTTP nunca é 4xx por falhas parciais de linha; o resultado por linha comunica os erros.

## Comportamento (EARS)

### Processamento normal

WHEN the operator POSTs `/api/games/{gameId}/achievements/import` with a non-empty `achievements` array,  
THEN the system SHALL process each item sequentially (in order),  
AND return HTTP 200 with a `results` array containing one entry per input row.

### Resultado por linha

| Status retornado do achievements-service | `status` no resultado |
|---|---|
| 201 Created | `"created"` |
| 409 Conflict (`achievement_exists`) | `"duplicate"` |
| 400 Bad Request (`invalid_achievement`) | `"invalid"` |
| 5xx / timeout / conexão recusada | `"error"` |

### Validações de entrada (rejeitam com 400)

- WHEN `achievements` está ausente ou é array vazio, THEN return 400 `"empty_batch"`.
- WHEN `achievements` tem mais de 500 itens, THEN return 400 `"batch_too_large"` (proteção contra abuso).
- WHEN um item tem campos desconhecidos (`DisallowUnknownFields`), THEN return 400 `"invalid_input"`.

### Processamento sequencial

IF alguma linha retornar erro (duplicate/invalid/error), a importação NÃO para — continua para a próxima linha (best-effort).

### Contadores de resumo

`created` = número de linhas com `status: "created"`.  
`failed`  = número de linhas com qualquer outro status.

## Não está no escopo desta spec

- Endpoint para receber o arquivo CSV diretamente (multipart) — o parsing fica no frontend.
- Rollback / atomicidade total (all-or-nothing) — foi deliberadamente escolhido best-effort para simplicidade.
- Rate limiting por endpoint.
- Autenticação/autorização (segue o padrão dos demais endpoints admin: sem auth no MVP, protegido por `X-Internal-Key` apenas na comunicação BFF → achievements-service).
