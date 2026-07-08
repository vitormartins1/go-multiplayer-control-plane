# Spec: 039-leaderboard-windows

**Serviço:** admin-ui
**Status:** Aprovado (2026-07-08)
**Data:** 2026-07-08
**Companheira de:** `leaderboard/007-time-windows` + `admin-bff/019`

> Fatia pequena/aditiva (UI sobre endpoint pronto): spec curta, sem plan/tasks.

## Comportamento

Na aba **Leaderboards** do jogo (que já tem seletor de board):

- Um **seletor de janela** (`SegmentedControl`: Geral · Hoje · Semana · Mês → `alltime|daily|weekly|monthly`) ao lado do seletor de board; default Geral. Trocar a janela refaz o fetch com `?window=`.
- **Paginação**: botão "Carregar mais" ao fim da tabela — busca a próxima página (`?offset=` += limit) e **anexa** as linhas (ranks contínuos vêm do serviço). Some quando a página vier menor que o limit.
- Janela periódica vazia → estado vazio atual com hint ("sem partidas no período").

## Mudanças

- `lib/api.ts`: `gameLeaderboard` ganha `window?` e `offset?`.
- `pages/GameLeaderboards.tsx` (ou componente equivalente da aba): SegmentedControl + estado de página/acúmulo; reset da lista ao trocar board/janela.

## Critérios de aceite

- [ ] Trocar janela refaz o fetch e zera a paginação
- [ ] "Carregar mais" anexa com ranks contínuos e some no fim
- [ ] Sem regressão no seletor de board (score/stats/cumulativos)
- [ ] Barra verde: `tsc` + `vite build`
