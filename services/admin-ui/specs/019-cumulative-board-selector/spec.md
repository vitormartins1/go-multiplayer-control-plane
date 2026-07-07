# Spec: 019-cumulative-board-selector

**Serviço:** admin-ui
**Status:** Draft
**Data:** 2026-07-07
**Parte de:** cauda da fatia A2 (`leaderboard/004-cumulative-boards`, ADR-0023). Estende o seletor de board da B2b (`017`).

## Contexto

A A2 criou boards cumulativos (`wins`, `<stat>_total`) além dos highscore. O seletor de board da aba Leaderboards (B2b) só listava `score` + os stats highscore. Esta cauda inclui os cumulativos nas opções, com rótulos que distinguem os modos.

## Comportamento

- THE SYSTEM SHALL, no seletor de board, listar as opções derivadas do perfil:
  - `score` → "Score";
  - cada stat `<s>` → "`<s>` (recorde)" (highscore, melhor de uma partida);
  - `wins` → "Vitórias (total)";
  - cada stat `<s>` → "`<s>` (total)" (cumulativo, board `<s>_total`).
- THE SYSTEM SHALL rotular a coluna de valor com o rótulo do board selecionado.
- Trocar de board recarrega o Top daquele board (inalterado da B2b).

## Contratos de Interface (cliente)

- `src/pages/GameLeaderboards.tsx`: `boardOptions` (lista `{value,label}`) inclui `wins` e `<stat>_total`; `boardLabel` para o cabeçalho da coluna. Sem mudança em `api.ts` (a leitura por `?board` já existe).

## Casos de Teste / Verificação

- [ ] jogo com `scoringStats` → seletor lista recorde + totais + Vitórias
- [ ] escolher "Vitórias (total)" mostra o ranking de vitórias; coluna rotulada
- [ ] `npm run build` (typecheck) limpo

## Fora do escopo

- Rankings por time; seasons; distinguir visualmente os modos além do rótulo.
