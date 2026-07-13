# Spec: 073-leaderboards-section-headers

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071/072 (sessão de exploração/ajuste visual).

## Contexto

Revisão da aba Leaderboards (pedida pelo usuário depois do polish da
Analytics e da padronização de densidade das tabelas, spec 072). O modo
"Visão geral" empilha até 5 grupos de board (Score, `<stat>` (recorde),
Vitórias (total), `<stat>` (total) — um por `scoringStat` declarado),
cada um com 4 mini-tabelas (Geral/Hoje/Semana/Mês). O separador de cada
grupo era um `<Divider label={...} labelPosition="left" />` — um traço
fino com texto pequeno, insuficiente pra ancorar a leitura ao rolar uma
página que pode ficar bem longa (jogos com vários `scoringStats`).

O resto do console já tem uma convenção estabelecida pra cabeçalho de
subseção dentro de uma página — `<Title order={5}>` sem Divider ao
redor (Overview.tsx: "Saúde dos serviços"/"Matchmaking"/"Fan-out /
entrega"; Analytics.tsx: "Partidas por dia"/"Jogos por partidas"). O
Divider com label só aparecia em mais um lugar no projeto inteiro
(MultiplayerSettings.tsx, contexto não relacionado) — não era um padrão
deliberado, só a primeira solução que apareceu quando o modo "Visão
geral" foi escrito.

## Comportamento

- `GameLeaderboards.tsx`: cabeçalho de cada grupo de board no modo
  "Visão geral" passa de `Divider label` pra `Title order={5}`, igual à
  convenção já usada em Overview/Analytics.
- Sem mudança de dado ou comportamento — só o cabeçalho visual.

## Fora de escopo

- Toolbar da aba (3 `SegmentedControl` + botão Atualizar): já testado
  em largura estreita (1024px) — o `Group` quebra de linha
  graciosamente (o botão desce, os controles continuam legíveis). Não é
  um problema real, não mexido.
- Modo "Específica" (tabela única): já padronizado na spec 072.

## Critérios de Aceite

1. Modo "Visão geral" (single board ou "Todas"): cada grupo de board
   tem `Title order={5}` como cabeçalho, sem Divider.
2. Sem regressão visual no modo "Específica" (não tocado).
3. Sem erro de console (validado com Playwright, Packet Arena, 4 boards
   × 4 janelas = 16 mini-tabelas).
