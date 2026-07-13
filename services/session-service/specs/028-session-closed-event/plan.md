# Plan — 028 evento de fechamento de sessão

## Decisões técnicas

1. **Um único kind novo (`session_closed`) pra todos os arquétipos e
   motivos** — ao contrário de `win_loss_draw`/`placement`/`collective`
   (polimórficos porque o RESULTADO tem formato diferente por arquétipo), o
   dado do FECHAMENTO em si (duração, motivo) é idêntico nos 4 arquétipos.
   Uma spec, um envelope, sem repetir o padrão 4x.
2. **`archetype` vai no envelope** (diferente dos outros kinds, que não
   precisam — o `kind` já entrega essa informação implicitamente). Aqui
   `kind=session_closed` sozinho não diferencia arquétipo, e o
   analytics-service precisa dessa dimensão pra agrupar/exibir (mesmo
   motivo que `postgres_history.go` já extrai `sess.Profile.Archetype`).
3. **session-service publica os timestamps crus (`openedAt`/`closedAt`),
   não a duração pré-computada** — mesma filosofia de deixar o consumidor
   derivar o que ele pode (padrão já usado: `StatSummary.Avg` é computado
   no analytics-service a partir de `Sum`/`Count`, não em quem publica).
4. **Publish DEPOIS de `persistHistory`, mesmo bloco best-effort** —
   reaproveita o ponto onde `sess` (com `ClosedAt`/`CloseReason` já
   setados) está disponível; falha do publish nunca desfaz o close nem o
   persistHistory (idêntico ao já estabelecido).
5. **Option nil-safe (`WithSessionClosedPublisher`)** — ambiente de dev sem
   fan-out configurado (só `LogPublisher`, se for o caso) continua
   funcionando; a Option é só mais um adapter que pode ou não estar presente.

## Arquivos por camada

- `internal/domain/session.go`: `SessionClosedPublisher` interface (nova).
- `internal/service/session_service.go`:
  - `sessionClosedPublisher domain.SessionClosedPublisher` (campo novo,
    zero value = nil = não configurado).
  - `WithSessionClosedPublisher(p domain.SessionClosedPublisher) Option`.
  - `publishSessionClosed(ctx, sess *domain.Session)` — mesmo formato de
    `persistHistory`: nil-check, loga + incrementa
    `sessionClosedPublishFailures` em erro, nunca propaga.
  - `SessionClosedPublishFailures() float64` (CounterFunc, mesmo padrão de
    `HistoryPersistFailures`).
  - `closeSession`: chama `s.publishSessionClosed(ctx, sess)` logo após
    `s.persistHistory(ctx, sess)`.
- `internal/publisher/sns_publisher.go`:
  - `kindSessionClosed = "session_closed"` (const).
  - `sessionClosedEnvelope` struct (schema do spec.md).
  - `PublishSessionClosed(ctx, sess *domain.Session) error` — extrai
    `archetype` de `sess.Profile` (nil-safe: `sess.Profile` sempre presente
    quando a sessão chegou a abrir, mas mantém o padrão defensivo de
    `postgres_history.go`), formata `openedAt`/`closedAt` em RFC3339,
    `json.Marshal` + `sns.Publish` (mesmo formato dos outros 4 métodos).
- `internal/publisher/log_publisher.go`: `PublishSessionClosed` — loga
  (mesmo padrão dos outros métodos do fallback de dev).
- `cmd/server/main.go`: wire `WithSessionClosedPublisher(publisher)` junto
  dos outros (mesmo adapter SNS/Log já satisfaz todas as Ports).
- Testes: `session_service_test.go` (closeSession publica; publisher nil
  não quebra; falha de publish não impede o close), `sns_publisher_test.go`
  (envelope correto, `archetype` presente).

## Ordem de implementação

Domínio (`SessionClosedPublisher`) → service (Option + `publishSessionClosed`
+ wiring em `closeSession`) → publisher SNS + Log → `main.go` (wiring) →
testes → build/test → validação ao vivo (fechar sessões dos 4 arquétipos por
motivos diferentes, grpcurl + `docker logs` ou inspeção da fila SQS local,
confirmar o envelope certo por caso).
