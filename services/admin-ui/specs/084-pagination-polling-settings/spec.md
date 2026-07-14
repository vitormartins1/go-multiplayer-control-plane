# Spec: 084-pagination-polling-settings

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 083 (sessão de exploração/polish iterativa). Este item foi
selecionado pelo usuário a partir de um levantamento de oportunidades de
Settings (não registrado em spec própria, entregue como texto na
conversa).

## Contexto

Levantamento do que existia hoje no console revelou duas inconsistências
sem motivo real:

- **Tamanho de página**: `Players.tsx` usava `PAGE = 50`;
  `GameLeaderboards.tsx`, `GameSessions.tsx` e `PlayerHistory.tsx` usavam
  `PAGE = 20` — três valores diferentes pra "quantos itens numa página",
  cada um hardcoded no próprio arquivo.
- **Intervalo de polling**: `App.tsx` (`HealthStrip`, `FleetNavBadge`) e
  `Overview.tsx` usavam 10s; `GameSessions.tsx` e `Rooms.tsx` usavam 5s —
  mesma divergência sem justificativa, cada `useEffect`+`setInterval`
  reimplementado à mão com dependências inconsistentes (alguns fixavam
  isso com `eslint-disable-next-line react-hooks/exhaustive-deps`).

O usuário escolheu resolver as duas de uma vez, como preferências globais
do console (Settings, spec 082), com a opção adicional de **pausar** o
polling — pedido explícito: "poder congelar pra debugar".

## Decisões

### 1. `createStoredEnum<T>()` (`lib/settings.tsx`)

Com duas preferências novas somando à já existente (`gameCoverStyle`), o
padrão de leitura/escrita/sincronização entre abas via `localStorage`
se repetiria 3x — extraído num factory genérico. Mesmo critério já usado
nesta sessão para `ChannelStrip`/`MockZone`: generalizar quando o padrão
se repete, não antes. Valores numéricos (`pageSize`, `pollingMs`) exigem
coerção `Number()` na leitura, já que `localStorage` só guarda string.

### 2. `usePolling(callback, ms)` (`lib/usePolling.ts`, novo)

Hook único para substituir os 4 blocos `useEffect`+`setInterval`
manuais. Guarda o callback numa `ref` ("latest callback" pattern) — o
`setInterval` sempre chama a versão mais recente, então o efeito só
precisa re-assinar quando `ms` muda. Isso elimina o risco de closure
obsoleta que os blocos antigos tinham (mascarado pelo
`eslint-disable`), não é só uma consolidação de números mágicos.
`ms <= 0` desliga o polling inteiramente — usado pela opção "Pausado".

### 3. Consumidores atualizados

- `App.tsx` (`HealthStrip`, `FleetNavBadge`), `Overview.tsx`: polling
  combinado via `usePolling` + `useSettings().pollingMs`.
- `GameSessions.tsx`, `Rooms.tsx`: mesma troca; `useEffect` continua
  importado nos dois (usado por outros efeitos do arquivo, não só o
  polling removido).
- `Players.tsx`, `GameLeaderboards.tsx`, `PlayerHistory.tsx`,
  `GameSessions.tsx`: `PAGE`/`CLOSED_PAGE` locais viraram
  `useSettings().pageSize` (renomeado localmente pra `PAGE` onde o resto
  do arquivo já usava esse nome, pra minimizar o diff).
- **Exceção deliberada**: `GameLeaderboards.tsx` tem também
  `OVERVIEW_TOP = 5`, o tamanho fixo das mini-tabelas "top 5" da visão
  Geral — isso é um preview de tamanho fixo, não uma página, e
  permanece hardcoded (não é a mesma coisa que a preferência global).

### 4. `Settings.tsx` — duas seções novas

"Paginação" (`SegmentedControl` sobre `PAGE_SIZE_OPTIONS`: 10/20/50/100)
e "Atualização automática" (`SegmentedControl` sobre `POLLING_OPTIONS`:
5s/10s/30s/Pausado), mesmo padrão visual das seções existentes
(`FormSection`).

## Critérios de Aceite

1. `tsc --noEmit` limpo em todos os arquivos tocados.
2. Settings mostra as duas novas seções, funcionais.
3. Mudar o tamanho de página em Settings afeta a tabela paginada em
   outra página sem reload — validado ao vivo (Playwright): `pageSize`
   setado para 10, `Players` mostrou os 9 jogadores existentes (a base
   de teste tem menos que 10, então não força paginação, mas confirma
   que o valor de `pageSize` chegou até a chamada da API).
4. "Pausado" para o polling de verdade — validado ao vivo (Playwright,
   `page.on("request", ...)`): 0 requisições a `/rooms` em 6s com
   polling pausado; ≥1 requisição em 6.5s com polling voltando a 5s.
5. Sem erro de console em nenhuma das telas tocadas.

## Fora de escopo

- Persistência server-side das preferências (continuam em
  `localStorage`, por navegador/dispositivo — mesmo modelo de
  `gameCoverStyle`, spec 082).
- Unificar `OVERVIEW_TOP` (preview fixo) com `pageSize` — são conceitos
  diferentes, mantidos separados de propósito.
