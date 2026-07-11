# Spec: 012-game-settings

**Serviço:** admin-ui
**Status:** Done (2026-07-04)
**Data:** 2026-07-04
**Parte de:** Console — Configurações do jogo (pares: `game-registry/005`, `admin-bff/010`)

## Contexto

A aba **Configurações** do GameDetail é ComingSoon. Substitui pelo **rename** do jogo (o único write de config disponível; delete adiado).

## Comportamento

- THE SYSTEM SHALL, na aba Configurações, mostrar um form com o **nome** atual (editável) e o **slug** (somente leitura, é a chave estável). Salvar chama `PUT /api/games/{gameId}` `{name}`.
- Sucesso -> notificação verde + recarrega a lista de jogos (título/breadcrumb refletem o novo nome). Erro (validação/serviço) -> notificação vermelha. Botão desabilitado quando o nome está vazio ou igual ao atual.

## Contratos de Interface (cliente)

- `src/lib/api.ts`: `api.renameGame(gameId, name)` -> `PUT /api/games/{gameId}`.
- `src/pages/GameSettings.tsx` (novo): form (`TextInput` nome + slug read-only + Save). Props `{ game, onRenamed }`.
- `src/pages/GameDetail.tsx`: captura `reload` da lista de jogos; renderiza `GameSettings` na aba (ou loader enquanto o jogo não carregou); remove o ComingSoon.

## Casos de Teste / Verificação

- [ ] editar o nome + salvar -> notificação, título/breadcrumb atualizam
- [ ] nome vazio/igual -> Save desabilitado; erro do serviço -> notificação vermelha
- [ ] `npm run build` (typecheck) limpo

## Fora do escopo
- Delete de jogo; editar slug/capa.
