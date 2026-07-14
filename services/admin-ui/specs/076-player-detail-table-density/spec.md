# Spec: 076-player-detail-table-density

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 075.

## Contexto

Revisão pedida pelo usuário da página de detalhe do jogador (`/players/
:id`, abas Achievements/History/Player data/Cloud saves), com exemplos
reais: Diego_Ping (Player Data + Cloud Save populados) e CamaradaDeFe1
(1 sessão no Histórico). As abas Player data e Cloud saves já estavam
bem cuidadas (JSON navegável + "leitura consolidada" lado a lado, spec
042) — nenhum problema encontrado nelas.

Duas tabelas ficaram fora da padronização de densidade das specs
072/074 (mesma causa raiz: ícone/avatar menor que os 32px de
referência, que os outros lugares já usam):

- `PlayerAchievements.tsx`: `AchievementIcon` com `size={28}` explícito
  (referência usa o default, 32px) → linha de 43px em vez de 47px.
- `PlayerHistory.tsx`: `PlayersSummary` sem `size` (usa o default 24px,
  mesmo antes de spec 072 corrigir `GameSessions.tsx`) → linha de 39px.

## Comportamento

- `PlayerAchievements.tsx`: remove o `size={28}`, cai no default (32px).
- `PlayerHistory.tsx`: `PlayersSummary` recebe `size={32}`, igual ao que
  já foi feito em `GameSessions.tsx` (spec 072).
- Sem mudança de dado/comportamento — só altura de linha.

## Critérios de Aceite

1. Achievements e History (aba do jogador) têm `rowHeight: 47px`, igual
   às outras tabelas do console.
2. Sem regressão nas abas Player data/Cloud saves (não tocadas).
3. Sem erro de console novo (o 404 de "sem cloud save" é esperado —
   REST semantics de recurso ausente, já tratado como estado vazio).
