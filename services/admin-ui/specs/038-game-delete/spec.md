# Spec: 038-game-delete

**Serviço:** admin-ui
**Status:** Rascunho
**Data:** 2026-07-08
**ADR:** 0025 (remoção de jogo — soft delete no registry)
**Companheira de:** `game-registry/010` + `admin-bff/018`

> Fatia pequena/aditiva (UI sobre endpoint pronto): spec curta, sem plan/tasks.

## Comportamento

Na aba **Configurações** do jogo (`GameSettings.tsx`), o aviso "Remover jogo ainda não está disponível" dá lugar a uma **zona de perigo**:

- WHEN o admin clica em "Remover jogo" → THE SYSTEM SHALL abrir um modal de confirmação exigindo **digitar o nome do jogo** (comparação exata, case-sensitive, com o nome atual); o botão destrutivo fica desabilitado até o texto casar.
- WHEN confirmado → `DELETE /api/games/{gameId}`; em `204` → notificação de sucesso + navegação para a biblioteca (`/games`); a biblioteca não mostra mais o jogo (dado vem do registry, já filtrado).
- IF `404` → notificação "jogo não encontrado" + navegação para a biblioteca (já era deletado).
- IF erro de rede/5xx → notificação de erro, modal permanece aberto (retry manual).
- O modal explica o efeito real (padrão do ADR-0025): o jogo some do console e para de aceitar filas/sessões novas; conquistas, saves, boards e analytics ficam inacessíveis; **não há desfazer pelo console**.

## Mudanças

- `lib/api.ts` (ou client equivalente): `deleteGame(gameId)`.
- `pages/GameSettings.tsx`: zona de perigo (Button `color="red"` variant outline) + modal de confirmação (Mantine `Modal` + `TextInput`), substituindo o `Alert` atual.

## Critérios de aceite

- [ ] Botão desabilitado até o nome digitado casar exatamente
- [ ] 204 → notificação + volta para a biblioteca (jogo ausente da lista)
- [ ] 404/erro → notificações distintas; erro mantém o modal aberto
- [ ] Barra verde: `tsc` + `vite build` (projeto sem test runner)
