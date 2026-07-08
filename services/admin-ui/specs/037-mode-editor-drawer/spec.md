# Spec: 037-mode-editor-drawer

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** lote console-ux-2. Pré-requisito prático do wizard (035): destaca o
`ModeEditor` como peça plugável.

## Contexto

Criar/editar um modo troca a view inteira da seção Multiplayer (lista → "página" do
editor com voltar). O Vitor levantou click-to-expand (accordion inline); avaliação:
os forms por arquétipo são longos (queue_arena tem 8+ campos condicionais) — inline
numa linha de tabela fica espremido e rolagem-dentro-de-rolagem. Recomendação
adotada: **Drawer à direita** — a lista permanece visível ao fundo (contexto), o form
ganha altura inteira da viewport, e o padrão já é o de consoles admin.

## Comportamento

- WHEN o usuário clicar em "Novo modo" ou no editar de um modo → THE SYSTEM SHALL
  abrir o `ModeEditor` num **Drawer** à direita (position right, size ~560px,
  título "Novo modo" / "Editar «modeId»"), mantendo a lista de modos visível.
- Salvar com sucesso → fecha o Drawer e recarrega a lista; Cancelar/fechar → só fecha
  (sem confirmar se não houve mudança; o Drawer do Mantine já fecha por Esc/overlay).
- O alerta de troca de arquétipo na edição (026) e o conteúdo do editor permanecem;
  o cabeçalho interno do editor (026: voltar + ações no topo) saiu — o Drawer provê
  título e fechar, e o form mantém os botões no rodapé.

## Contratos de Interface (cliente)

- `MultiplayerSettings.tsx`: o state `view {list|editor}` vira `editorTarget:
  null | { mode?: MultiplayerMode }` controlando um `<Drawer>`; `ModeEditor` intacto
  (o refactor de submissão plugável fica na 035, que é quem precisa dele).

## Casos de Teste / Verificação

- [x] criar e editar abrem no Drawer com a lista visível ao fundo
- [x] salvar fecha e atualiza a lista; Esc/overlay fecham sem salvar
- [x] alerta de arquétipo (026) segue funcionando na edição
- [x] `npm run build` limpo

## Fora do escopo

- Accordion/inline expand (rejeitado — forms longos); dirty-check com confirmação ao
  fechar; edição em massa.
