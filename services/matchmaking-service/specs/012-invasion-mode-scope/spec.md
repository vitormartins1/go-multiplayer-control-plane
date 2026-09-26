# Spec: 012-invasion-mode-scope

**Serviço:** matchmaking-service (+ session-service, proto `session.v1`)
**Status:** Done
**Data:** 2026-09-26
**Decisão:** D2 da revisão de produto 2026-09 (`docs/review/REVISAO-2026-09.md` §6), aprovada pelo dono.
**Contrato:** mudança **compatível** do `.proto` (campo novo opcional); muda quem encontra quem no `Invade`.

## Contexto

O `Invade` (spec 006) usa o `mode_id` do invasor só para ler a política do modo; a lista de mundos vem de
`session.ListInvadableWorlds(game_id)`, **sem modo**. Um invasor do modo A podia ser injetado num mundo aberto no
modo B — com a política (faixas, senha, slots) de A aplicada a um mundo de B. O quadro de sinais (spec 007) já é
separado por modo, e o contrato do `host_world_dropin` diz que o modo "flui pelo Invade".

## Comportamento

- `ListInvadableWorldsRequest` ganha `string mode_id = 2`.
  - WHEN `mode_id` vem preenchido -> THE SYSTEM (session) SHALL devolver só mundos abertos nesse modo.
  - WHEN `mode_id` vem vazio -> todos os modos do jogo (comportamento anterior; compat para chamadores antigos).
- WHEN o jogador chama `Invade(game_id, mode_id)` -> THE SYSTEM (matchmaking) SHALL listar só os mundos do modo
  normalizado do invasor (vazio = `"default"`, como no resto do ADR-0024) e aplicar a política desse modo.
- Senha (spec 011) continua valendo **dentro do modo**: amigo com a senha certa precisa estar no mesmo modo do host.

## Critérios de aceite

1. session: `ListInvadableWorlds` com modo filtra; sem modo devolve todos (teste de service e de transport).
2. matchmaking: `Invade` repassa o modo normalizado ao diretório (teste do invader e do client gRPC).
3. Contrato `host_world_dropin-dataplane-contract.md` e `docs/contracts/CHANGELOG.md` registram a mudança e o
   impacto no jogo: **o jogo precisa mandar no `Invade` o mesmo `mode_id` com que o host abriu o mundo** (já é o
   que o contrato pede; quem mandava vazio passa a achar só mundos do modo `default`).

## Fora de escopo

Invasão entre modos de propósito (ex.: um modo "arena" que invade qualquer mundo) — só com caso real.
