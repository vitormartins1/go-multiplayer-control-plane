# Spec: 035-game-create-wizard

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** lote console-ux-2. Fatia substancial → tem `plan.md` e `tasks.md`.
Depende da 037 (ModeEditor plugável via Drawer é meio caminho).

## Contexto

Registrar um jogo hoje é um modal nome+slug; o multiplayer é configurado depois, em
outra aba, e a capa gerada só aparece após criar. A jornada proposta (Vitor) guia o
cadastro completo em steps com preview — e só toca a API na confirmação final (nenhum
recurso órfão se o wizard for abandonado).

## Comportamento

- WHEN o usuário clicar em "Registrar jogo" → THE SYSTEM SHALL abrir um wizard
  (Mantine `Stepper`, modal grande) com 3 passos:
  1. **Identidade** — Nome e Slug (slug sugerido automaticamente a partir do nome,
     editável, com a regra/aviso de imutabilidade) + **preview ao vivo da capa
     gerada** (GameCover com um Game sintético a partir do slug; muda ao digitar).
  2. **Multiplayer (opcional)** — configurar o primeiro modo com o MESMO `ModeEditor`
     das Configurações (em modo "coletar", sem salvar na API); botão "Pular" declara
     o jogo sem multiplayer por ora.
  3. **Revisão** — capa + nome/slug + resumo do modo (`modeSummary`) ou "sem
     multiplayer"; botão **Confirmar** executa: `POST /games` e, se houver modo,
     `PUT /games/{id}/multiplayer-profiles/{modeId}`.
- Nenhuma chamada de escrita antes do Confirmar. Sucesso → navega para o GameDetail
  do jogo novo. Falha no POST → erro no próprio wizard (slug duplicado aponta o
  passo 1). Falha só no PUT do modo → jogo foi criado: notificação explica e aponta
  Configurações (sem rollback — o jogo é válido sem modo).
- Voltar entre passos preserva os valores.

## Contratos de Interface (cliente)

- `Games.tsx`: modal atual substituído pelo wizard (`GameCreateWizard.tsx` novo).
- `MultiplayerSettings.tsx`: `ModeEditor` ganha prop de submissão plugável
  (`onSubmit(modeId, displayName, profile)`) — o caller decide salvar na API
  (Configurações/037) ou coletar em memória (wizard). Export do componente.
- `GameCover`: aceita Game sintético (client-side puro, já é o caso).

## Casos de Teste / Verificação

- [x] preview da capa muda ao digitar o slug; slug sugerido do nome, editável
- [x] abandonar o wizard em qualquer passo não cria nada (sem órfãos)
- [x] confirmar com modo → jogo + modo criados; sem modo → só o jogo
- [x] slug duplicado → erro apontando o passo 1; PUT do modo falhou → jogo criado +
  notificação direcionando às Configurações
- [x] `npm run build` limpo

## Fora do escopo

- Múltiplos modos no wizard (um só; demais nas Configurações); editar jogo existente
  via wizard; upload de capa; validação server-side nova.
