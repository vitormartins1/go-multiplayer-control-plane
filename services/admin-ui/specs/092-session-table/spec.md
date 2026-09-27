# Spec: 092-session-table

**Serviço:** admin-ui
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** revisão de produto 2026-09 (Rooms e Sessões do jogo quase duplicados)

## Contexto

Rooms (board global) e a aba Sessões do jogo tinham cada uma a sua cópia da linha de sessão e do painel expandido
(~1000 linhas somadas). Cada mudança (hospedagem, heartbeat, P2P) precisava ser feita duas vezes e as duas já
divergiam: a aba do jogo não mostrava os slots por papel do mundo e o painel do Rooms repetia o id do mundo.

## Comportamento

- Um componente só, `SessionTable`, com colunas opcionais: **Jogo** e **Arquétipo** (Rooms), ordenação pelos
  cabeçalhos (Rooms, spec 060), botão **Encerrar** (Rooms) e rótulos extras do modo (lobby/time).
- A aba Sessões do jogo usa o mesmo componente sem essas opções e passa a mostrar os **slots por papel** dos mundos
  e o painel completo (hospedagem, heartbeat, aviso P2P).
- Painel expandido único: o mundo (`host_world`) mostra **MUNDO** (o session id) uma vez só; partida mostra
  **SESSION ID** e **MATCH ID**.
- O comparador de ordenação vira função pura em `lib/sessionSort.ts`, com testes.

## Critérios de aceite

`tsc`, `vitest` e `vite build` verdes; captura das duas telas com mundos reais.
