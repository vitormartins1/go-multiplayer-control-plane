# Spec: 029-host-killed-retry

**Serviço:** session-service
**Status:** Done
**Data:** 2026-09-26
**Nível:** fatia pequena (correção; sem mudança de proto nem de envelope)
**Origem:** revisão de produto 2026-09 (risco R2)

## Contexto

Desde o fix `49e976d` (2026-08-19), `ReportParticipationEvent` **muta antes de publicar** para que o gameplay não
fique refém do fan-out. Para `HOST_KILLED` a mutação é fechar o mundo — que o remove do mapa de sessões ativas.
Se o publish falha em seguida, o erro volta para o server, que reenvia com o mesmo `event_id`; mas o reenvio cai no
lookup inicial (mundo não está mais entre os ativos) e recebe `NotFound`, que o contrato manda tratar como
"pare de reenviar". Resultado: o `HOST_KILLED` **nunca chega** ao fan-out — o invasor perde o `invasion_won` e o
analytics perde o evento. O mesmo acontece quando o `Release` do GameServer falha dentro do fechamento (o erro
volta antes do publish).

## Comportamento

- WHEN um `HOST_KILLED` fecha o mundo -> THE SYSTEM SHALL guardar esse evento na sessão fechada (histórico em
  memória) como o **evento de fechamento**.
- WHEN chega um `ReportParticipationEvent` para um mundo que não está mais ativo E o evento é a **mesma
  ocorrência** do evento de fechamento (mesmo `event_id`; se o `event_id` vier vazio — data plane antigo —, mesmo
  `player_id` + `event`) -> THE SYSTEM SHALL **só republicar** no fan-out (sem nenhuma mutação) e responder como
  sucesso se o publish passar, erro se falhar (o server segue reenviando).
- Qualquer outro evento para um mundo que não está ativo continua recebendo `NotFound` (comportamento atual).
- Republicar uma ocorrência já publicada é seguro: os sinks deduplicam por `event_id` (at-least-once, ADR-0020).

## Critérios de aceite

1. Publish falha no `HOST_KILLED` → o mundo fecha mesmo assim; o reenvio do mesmo evento publica e retorna OK.
2. `Release` falha no `HOST_KILLED` → o reenvio publica e retorna OK.
3. Outro evento (ex.: `LEFT` de um phantom) para o mundo fechado → `ErrWorldNotFound`.
4. `HOST_KILLED` com outro `event_id` para o mundo fechado → `ErrWorldNotFound`.
5. Contrato (`host_world_dropin-dataplane-contract.md`), comentário do service e do `.proto` descrevem a ordem real
   (muta, depois publica) e a semântica do reenvio.

## Fora de escopo

- Persistir mundos abertos (restart do session-service ainda perde mundos vivos).
- Outbox para o publish (a garantia continua sendo o reenvio do server).
- A janela do reenvio é o histórico em memória (últimas 200 sessões fechadas por padrão); sessão que já saiu dele
  volta a responder `NotFound`.
