# Spec 014 — Comparadores no match das event rules (V2a do ADR-0028)

**Status:** Draft — aguardando aprovação
**Serviço:** achievements-service (domain + service + repository + migration);
admin-ui (render + form de operador no canvas/tabela)
**Relaciona:** ADR-0028 (decisão + adendo — V2a é a etapa 1 do motor rico),
ADR-0027 (formato atual do match), spec 013 (fact `mode`)

## Contexto

O match atual é igualdade exata (`map[string]string`). O único limiar numérico
do sistema (`top_n <= 10` do battle_royale) vive hardcoded na normalização
(`BattleRoyaleTopNThreshold` + fact derivado `tier`) — parâmetro de arquétipo
preso em código Go, contra o princípio "por cadastro, não por código". O V2a
introduz comparadores preservando 100% das regras existentes, e é pré-requisito
do stats-as-facts (condições compostas como *flawless victory*).

## Formato do match (JSONB, retrocompatível)

O valor de cada chave passa a aceitar **duas formas**:

```jsonc
{
  "outcome": "win",                                   // string  = igualdade (formato atual, intocado)
  "placement": { "op": "lte", "value": 10 },          // objeto  = comparador
  "mode": { "op": "in", "value": ["ranked", "arena"] }
}
```

**Operadores** (conjunto mínimo, por evidência):

| op | value (JSON) | semântica |
|---|---|---|
| — (string) | string | igualdade exata (como hoje) |
| `lt` `lte` `gt` `gte` | number | comparação numérica: o fact é parseado como número; fact ausente ou não-numérico **não casa** |
| `ne` | string | diferente (fact ausente **não casa** — `ne` exige a chave presente) |
| `in` | array de strings | fact pertence à lista |

## Comportamento

- **Domain:** `EventRule.Match` vira `map[string]MatchCond` (`Op` + `Value`).
  `UnmarshalJSON` aceita as duas formas (string → `eq`); `MarshalJSON` emite
  string pura para `eq` (formato compacto preservado no banco e na API).
  `matchSubset` ganha o switch por operador — continua função pura.
- **Validação na criação** (`CreateEventRule`): op desconhecido, value de tipo
  errado ou lista vazia no `in` → `ErrInvalidEventRule` (400). Regra global
  segue read-only.
- **Avaliação defensiva:** op desconhecido vindo do banco (dado antigo de
  versão futura?) **não casa** e loga warning — nunca casa silenciosamente,
  nunca derruba o processamento do envelope.
- **Migration 009:** a regra global do battle_royale troca
  `{"tier":"top_n"}` por `{"placement":{"op":"lte","value":10}}`; a derivação
  do fact `tier` e a constante `BattleRoyaleTopNThreshold` **saem do código**
  (o limiar vira dado editável — objetivo central do V2a). Overrides de jogo
  que usem `tier` (nenhum conhecido) são migrados pelo mesmo UPDATE.
- **API admin:** mesmos endpoints; o payload de criação aceita o formato novo
  (o BFF é passthrough, sem mudança).
- **Admin-ui:**
  - Render (canvas + tabela + painéis): condição não-eq exibida como
    `placement ≤ 10`, `mode in [ranked, arena]` (badge/nó por condição
    distinta — o id do nó de fact passa a incluir op+value).
  - Form de criação (drawer): cada par ganha um select de operador
    (`= ≠ < ≤ > ≥ in`, default `=`); value numérico para ops numéricos,
    lista (vírgula/tags) para `in`.

## Fora de escopo

- Operadores lógicos entre chaves (OR/NOT de grupos — o OR entre regras já é
  implícito; grupos são evolução futura por evidência).
- Stats-as-facts (etapa seguinte do adendo — depende deste V2a).
- Supressão de regra global por jogo (V2b) e prioridade (V2c).

## Critérios de aceite

1. **Zero regressão:** todas as regras existentes (strings) seguem casando
   idêntico; suíte atual passa sem mudança de expectativa (fora o `tier`).
2. **top_n data-driven:** com a migration 009, envelope `placement=10` casa
   `battle_royale_top_n` e `placement=11` não — provado por teste de domínio
   e e2e via SQS (mesmo método da spec 013); nenhuma referência a
   `BattleRoyaleTopNThreshold`/`tier` sobra no código.
3. Criação com op inválido/value de tipo errado → 400; op desconhecido no
   banco → não casa + warning logado.
4. Canvas renderiza e cria condições com operador; `npm run build` e
   `go test ./... -race` verdes.
