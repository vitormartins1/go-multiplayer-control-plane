# Spec: 029-catalog-consistency

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-07)
**Data:** 2026-07-07
**Parte de:** lote console-insights. Continuação da 027 (mesmos dados, agora como diagnóstico).
Decisão de design mantida: **detectar drift, não impedir** — aviso read-only, nunca bloqueio.

## Contexto

Uma conquista por métrica cujo `statKey` nenhum modo declara (ou por evento com `eventType`
fora do vocabulário dos arquétipos cadastrados) provavelmente é typo ou resto de um rename —
mas pode ser intencional (o envelope de stats é aberto). O console deve apontar a
inconsistência sem tratá-la como erro.

## Comportamento

- WHEN os modos do jogo carregarem E houver ≥1 modo → THE SYSTEM SHALL marcar na tabela de
  conquistas, junto ao critério:
  - critério por métrica com `statKey` fora da união `{score} ∪ scoringStats` dos modos →
    badge de aviso "não declarado" (amarelo, outline) com tooltip: o stat não consta dos
    modos; o data plane ainda pode reportá-lo, mas confira se não é typo/rename.
  - critério por evento com `eventType` fora do vocabulário dos arquétipos cadastrados
    (tabela `EVENT_VOCABULARY` da 027) → badge "fora do vocabulário" com tooltip análogo.
- WHEN os modos não carregarem OU o jogo não tiver modos → THE SYSTEM SHALL não marcar nada
  (sem falso alarme).
- O aviso é puramente informativo: não altera validação, submit ou import CSV.

## Contratos de Interface (cliente)

- `GameAchievements.tsx`: helpers puros `statDeclared(key, modes)` / `eventKnown(type, modes)`
  reutilizando as derivações da 027; badge + Tooltip na célula "Critério".

## Casos de Teste / Verificação

- [x] conquista com stat declarado num modo → sem aviso; stat órfão → badge "não declarado"
- [x] conquista `score` → sem aviso (intrínseco)
- [x] evento do vocabulário de arquétipo cadastrado → sem aviso; fora dele → badge
- [x] jogo sem modos ou registry fora → nenhum badge
- [x] `npm run build` limpo

## Fora do escopo

- Checagem inversa global (stat declarado sem uso) — sinal fraco, leaderboards nascem por
  stat declarado de toda forma; validação server-side; painel dedicado de consistência.
