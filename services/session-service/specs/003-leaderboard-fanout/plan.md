# Plan: 003-leaderboard-fanout
**Spec:** specs/003-leaderboard-fanout/spec.md
**Data:** 2026-06-30

## Abordagem técnica
Novo adapter `Leaderboard` (irmão do `Achievements`): POST do score por jogador
no leaderboard, best-effort, internal key no client. Novo `Composite` que
satisfaz `ResultPublisher` e delega para uma lista (fan-out N sinks). `main` monta
a lista conforme as envs de URL e injeta o Composite (ou Log se vazia).

## Arquivos
- `internal/publisher/leaderboard_publisher.go` (+ test httptest) — `Publish` envia `r.Score` por jogador a `/admin/games/{g}/players/{p}/scores`; best-effort
- `internal/publisher/composite.go` (+ test) — `Composite` delega para N publishers; loga falhas, não aborta
- `cmd/server/main.go` — `buildPublisher` monta `[]ResultPublisher` (achievements se `ACHIEVEMENTS_URL`, leaderboard se `LEADERBOARD_URL`); client único com `auth.InternalKeyTransport`; Composite (ou Log se vazio)
- `infra/docker/docker-compose.yml` — session: `LEADERBOARD_URL` + depends_on leaderboard
- `api/README.md` — nota: fan-out p/ achievements + leaderboard

## Decisões
- **Composite** (fan-out N sinks) vs campos separados: `ReportResult` segue com um `Publish`; novos destinos entram na lista.
- **Best-effort por sink**: cada adapter já retorna nil em falha (loga); o Composite também não aborta entre sinks.
- **Score = `r.Score`** direto (sem normalização no MVP).
- **Client compartilhado** entre os adapters (mesma internal key).

## Ordem
1. LeaderboardPublisher + httptest
2. Composite + test
3. main (lista + composite) + compose
4. `go test -race` + build; validar o loop no Docker: ReportResult -> leaderboard top-N do jogo reflete os scores

## Riscos
- **Dois sinks, um lento**: best-effort isola; timeouts no client (já 5s).
- **Ordem no Composite**: irrelevante (sinks independentes).
