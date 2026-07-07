# Spec: 023-multiplayer-modes-console

**Serviço:** admin-ui
**Status:** Rascunho — aguardando aprovação
**Data:** 2026-07-07
**ADR:** 0024 · **Roadmap:** `multiplayer-modes-roadmap.md` — fatia **M0** (lado console, UI)

## Contexto

Fecha o M0: a aba **Configurações** do jogo deixa de editar **um** perfil e passa a gerenciar a **coleção de modos** (ADR-0024). O form por arquétipo existente (`MultiplayerSettings.tsx` — select de arquétipo + Fieldsets condicionais dos 4 arquétipos) é **reusado como o editor de um modo**; o que muda é a moldura em volta dele (lista → selecionar/criar/remover modo). Consome as rotas novas do BFF (spec `admin-bff/017`).

## Comportamento

### Lista de modos (aba Configurações, seção "Multiplayer")

- WHEN a aba carrega → THE SYSTEM SHALL buscar `GET /api/games/{id}/multiplayer-profiles` e listar os modos: `displayName` (ou `modeId` se vazio) + badge do arquétipo + `modeId` em monospace. `default` sempre primeiro (ordem do backend).
- Jogo sem modos → estado vazio com CTA "Configurar multiplayer" que abre o editor com `modeId = default` pré-preenchido e travado (preserva o fluxo atual de primeiro cadastro).
- Cada item tem ações **Editar** e **Remover**; o `default` não mostra Remover (regra do backend: 409).

### Criar/editar um modo

- WHEN "Novo modo" → editor com: campo `modeId` (obrigatório, hint do formato `^[a-z][a-z0-9-]{0,31}$`, validação client-side), campo `displayName` (opcional, ≤ 60), e o form de perfil existente (select de arquétipo + params por arquétipo, **inalterado**).
- WHEN "Editar" → mesmo editor com `modeId` **read-only** (id é estável; renomear exibição = displayName).
- WHEN salvar → `PUT /api/games/{id}/multiplayer-profiles/{modeId}` com `{displayName?, archetype, topology, params}`; sucesso → volta à lista com notificação; erro → mensagem mapeada (`invalid_mode_id`, `too_many_modes`, `invalid_profile_params`, …).

### Remover um modo

- WHEN "Remover" → modal de confirmação (nome do modo + aviso de irreversível) → `DELETE .../{modeId}` → 204 remove da lista. `default_mode_protected` (409) não deve ocorrer (botão oculto), mas se ocorrer mostra o erro.

### Compat

- O restante do console que lê o perfil (Rooms/Analytics/badges) segue usando o que já usa (perfil default via `GET /games/{id}`/alias) — **sem mudança nesta fatia**; a segmentação por modo no restante do console é pós-M1.

## Contratos (api.ts)

- Tipos: `MultiplayerMode = { modeId: string; displayName?: string } & MultiplayerProfile` (reusa o union `MultiplayerProfile` existente); `ModeSummary = { modeId, displayName?, archetype }`.
- Client: `listModes(gameId)`, `putMode(gameId, modeId, body)`, `deleteMode(gameId, modeId)`.
- `Game` ganha `multiplayerModes: ModeSummary[]` (aditivo — já vem no `GET /games/{id}`).

## Critérios de aceite

- [ ] Lista renderiza modos com badge de arquétipo; vazio → CTA default.
- [ ] Criar modo novo (modeId + displayName + perfil) e editar existente (modeId travado) funcionam ponta a ponta.
- [ ] Remover com confirmação; `default` sem botão de remover.
- [ ] Form por arquétipo existente reusado sem fork (um só componente de editor de perfil).
- [ ] Erros do backend aparecem como mensagens claras (validação client-side de modeId antes do PUT).
- [ ] Testes unitários do client novo + `npm run build` verde.

## Fora do escopo

- `mode_id` no matchmaking/ticket/token (M1) e segmentação do restante do console por modo (pós-M1).
- Reordenação/duplicação de modos; ícone por modo.
