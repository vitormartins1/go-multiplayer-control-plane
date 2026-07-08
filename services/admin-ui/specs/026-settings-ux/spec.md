# Spec: 026-settings-ux

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-07)
**Data:** 2026-07-07
**Parte de:** lote console-polish. Refina a aba Settings (GameSettings + MultiplayerSettings)
sem mudar o esqueleto (Fieldsets 2 colunas + tooltips ficam).

## Contexto

Quatro fricções observadas: (1) o `ModeEditor` substitui a lista sem título nem caminho de
volta no topo; (2) trocar o arquétipo na edição descarta os params salvos sem aviso; (3) a
lista de modos não resume os parâmetros; (4) a seção Identidade não mostra a capa do jogo.

## Comportamento

- **Contexto no editor:** THE SYSTEM SHALL exibir no topo do `ModeEditor` um cabeçalho com
  botão voltar (seta) + título "Novo modo" ou "Editar «modeId»", e as ações Salvar/Cancelar
  também no topo (mantendo as do rodapé).
- **Aviso de troca de arquétipo:** WHEN o usuário, editando um modo existente, selecionar um
  arquétipo diferente do salvo → THE SYSTEM SHALL exibir um Alert inline (warning): trocar o
  arquétipo redefine os parâmetros e muda o contrato com o data plane. Não bloqueia.
- **Resumo na lista:** THE SYSTEM SHALL exibir na tabela de modos uma coluna "Configuração"
  com resumo curto por arquétipo, ex.: `FFA · 4 jogadores · primeiro a 10`,
  `2×5 · melhor de 3`, `Squad 4 · lobby 60`, `Time 4 · 3 missões`, `Co-op 2 + Invasor 1`.
- **Capa na Identidade:** THE SYSTEM SHALL exibir o preview da capa gerada (`GameCover`) ao
  lado dos campos nome/slug na seção Identidade.

## Contratos de Interface (cliente)

- `src/pages/MultiplayerSettings.tsx`: header do editor; flag `archetypeChanged`; função
  `modeSummary(mode)` pura para a coluna.
- `src/pages/GameSettings.tsx`: `GameCover` no Fieldset de identidade (precisa do `Game`
  completo — já é prop).

## Casos de Teste / Verificação

- [x] criar/editar mostram título e voltar; Salvar no topo funciona igual ao do rodapé
- [x] alert só aparece na edição com arquétipo ≠ salvo; some ao voltar ao original
- [x] resumo correto para os 4 arquétipos (casos: FFA, times, BR squad, coop, HWD)
- [x] capa visível na Identidade sem quebrar o grid em telas estreitas
- [x] `npm run build` limpo

## Fora do escopo

- Validação nova de formulário; reordenar/duplicar modos; delete de jogo.
