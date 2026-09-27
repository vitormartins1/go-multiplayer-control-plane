# Spec: 033-report-result-retry

**Serviço:** session-service
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** lacuna G7 da revisão 2026-09 ("retry após sucesso dá NotFound")

## Contexto

`ReportResult` publica no fan-out e depois fecha a sessão. Se o Dedicated Server reenviar um resultado que **já foi
aceito** (a resposta se perdeu na rede, ou o release do GameServer falhou depois do publish e voltou erro), a sessão
já saiu do mapa de ativas e o reenvio recebe `NotFound` — o servidor não tem como saber que deu certo.

## Comportamento

- **Quando** chegar `ReportResult` para um `match_id` que não está ativo mas está no histórico em memória fechado por
  `result`, o session **deve** responder sucesso sem publicar de novo (idempotente — os consumidores já deduplicam
  por `matchId`, mas não há por que gerar tráfego).
- Match desconhecido (nem ativo, nem fechado por resultado no histórico) continua `NotFound`.
- A janela é a do histórico em memória (`historyLimit`, 200 sessões), igual à spec 029.

Crash do processo entre publicar e fechar não é tratado aqui: o estado do session é em memória, e o sweeper de TTL
fecha o que sobrar; como o consumidor deduplica por `matchId`, um republish não duplica nada.

## Critérios de aceite

Unit: reenvio após sucesso → nil, sem segundo publish; match nunca visto → `ErrSessionNotFound`; sessão fechada por
TTL (não por resultado) → `ErrSessionNotFound`.
