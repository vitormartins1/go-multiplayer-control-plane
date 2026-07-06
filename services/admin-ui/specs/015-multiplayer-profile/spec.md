# Spec: 015-multiplayer-profile

**Serviço:** admin-ui
**Status:** Draft
**Data:** 2026-07-06
**Parte de:** B1 do roadmap `queue_arena` (ADR-0023). Pares: `game-registry/006`, `admin-bff/012`.

## Contexto

Torna o perfil de multiplayer (ADR-0023) **visível e editável** no console — hoje só via API. Nova seção **Multiplayer** na aba Configurações do GameDetail (que hoje só tem rename). Formulário com campos **condicionais**, espelhando a validação do game-registry/006. O servidor é a verdade da validação; o cliente só evita submissões obviamente inválidas.

## Comportamento

- THE SYSTEM SHALL, ao abrir a aba Configurações, carregar o perfil via `GET /api/games/{gameId}/multiplayer-profile`. `404` -> estado "não configurado" (form em branco com defaults + botão "Configurar multiplayer"). Perfil existente -> form preenchido.
- O form SHALL ter campos condicionais:
  - `archetype`: Select, só `queue_arena` por ora (preparado para crescer).
  - `structure`: SegmentedControl `ffa` | `teams`.
    - `ffa` -> `playerCount` (NumberInput, min 2).
    - `teams` -> `teamCount` (min 2) + `teamSize` (min 1).
  - `winCondition`: Select `first_to_score` | `highest_at_time` | `objective` | `elimination`.
    - `first_to_score` -> `scoreTarget` (min 1).
    - `highest_at_time` -> `durationSeconds` (min 1).
  - `matchStructure`: SegmentedControl `single` | `best_of_rounds`.
    - `best_of_rounds` -> `roundsToWin` (min 1).
  - `allowDraw`: Switch.
  - `scoringStats`: TagsInput; cada item casa `^[a-z][a-z0-9_]*$` (valida no cliente antes de enviar).
  - `topology`: Select `dedicated` | `p2p` (com tooltip: `p2p` reservado, não implementado — rejeitado pelo backend).
- Salvar chama `PUT /api/games/{gameId}/multiplayer-profile` montando `{archetype, topology, params}` só com os campos relevantes à seleção atual. Sucesso -> notificação verde. Erro (validação/serviço) -> notificação vermelha (usa a mensagem estruturada quando houver).
- Botão Salvar desabilitado enquanto salva ou quando há erro de validação client-side (ex.: stat fora do padrão).

## Contratos de Interface (cliente)

- `src/lib/api.ts`:
  - tipos `MultiplayerProfile` + `QueueArenaParams` (espelham game-registry/006).
  - `api.getMultiplayerProfile(gameId)` -> `GET`; **retorna `null` em 404** (não configurado).
  - `api.setMultiplayerProfile(gameId, profile)` -> `PUT`.
- `src/pages/MultiplayerSettings.tsx` (novo): a seção do form. Props `{ gameId }`.
- `src/pages/GameSettings.tsx`: renderiza `<MultiplayerSettings>` abaixo do bloco de rename (Divider entre eles).

## Casos de Teste / Verificação

- [ ] jogo sem perfil -> estado "não configurado"; configurar `ffa`/`first_to_score` -> salva, notificação verde
- [ ] alternar `ffa`<->`teams` e `winCondition` mostra/esconde os campos dependentes corretos
- [ ] `scoringStats` com item inválido -> bloqueia Save (validação client-side)
- [ ] erro do servidor (ex.: `p2p`) -> notificação vermelha
- [ ] `npm run build` (typecheck) limpo

## Fora do escopo

- Outros arquétipos; implementar `p2p` (slot).
- Ler `scoringStats` em outras abas (B2b/A1/A2 fazem isso depois).
