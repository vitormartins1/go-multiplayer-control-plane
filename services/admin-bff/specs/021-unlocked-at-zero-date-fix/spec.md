# Spec 021 — Corrigir data zero (0001) em achievements não desbloqueados

> Status: **implementada** (nit registrado em memória, corrigido a pedido
> direto do Vitor, 2026-07-10)
> Fatia: pequena/aditiva — bug fix isolado no admin-bff, sem mudança de
> contrato pro admin-ui (o campo já era opcional no tipo TS).

## Contexto

O drawer de progresso de achievement (e a lista de achievements do jogador)
mostravam "Desbloqueado em 0001-..." pra linhas que ainda estavam em
progresso, não desbloqueadas. Causa raiz: `domain.PlayerAchievement` e
`domain.AchievementProgressEntry` no admin-bff declaravam `UnlockedAt
time.Time \`json:"unlockedAt,omitempty"\`` — `omitempty` do `encoding/json`
só omite os "vazios" de tipos básicos, slice, map e ponteiro; **nunca**
omite um `struct` (incluindo `time.Time`), mesmo no zero-value. O
achievements-service já fazia certo (`*time.Time` no upstream), mas o
admin-bff desempacotava pro tipo não-ponteiro e reserializava sempre com o
campo presente.

## Comportamento (EARS)

1. `PlayerAchievement.UnlockedAt` e `AchievementProgressEntry.UnlockedAt`
   DEVEM ser `*time.Time` — `nil` quando não desbloqueado, omitido do JSON
   (`omitempty` agora funciona de verdade).
2. Quando desbloqueado, o valor DEVE continuar chegando ao front igual a
   antes (mesmo formato ISO, mesmo comportamento de exibição).

## Critérios de aceite

- [x] Achievement em progresso não traz `unlockedAt` no JSON (campo ausente,
      não `"0001-01-01T00:00:00Z"`).
- [x] Achievement desbloqueado continua trazendo a data real.
- [x] Teste de regressão cobrindo o caso "em progresso" (o que antes vazava
      a data zero); testes existentes atualizados pro tipo ponteiro.
- [x] `-race` verde no admin-bff.

## Fora de escopo

- Qualquer mudança no admin-ui (o tipo TS já era opcional — o bug era
  inteiramente do lado do admin-bff).
