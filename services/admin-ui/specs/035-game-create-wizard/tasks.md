# Tasks: 035-game-create-wizard

- [x] **T1** — `lib/slug.ts`: slugify puro (acentos, minúsculas, hífens) + testes mentais
  nos casos: "Neon Breach", "Jogo Ágil 2", vazio.
- [x] **T2** — `ModeEditor` plugável: prop `onSubmit` + rótulo do botão; Configurações
  injeta o save atual (zero mudança de comportamento; build verde antes de seguir).
- [x] **T3** — `GameCreateWizard.tsx`: Stepper com passo 1 (Identidade + preview da capa
  com Game sintético + slug sugerido/touched).
- [x] **T4** — passo 2 (ModeEditor em modo coleta + "Pular") e passo 3 (Revisão com
  capa/nome/slug/modeSummary + Confirmar com POST→PUT e tratamento de erro por etapa).
- [x] **T5** — `Games.tsx`: substituir o modal pelo wizard; navegação pós-sucesso.
- [x] **T6** — `npm run build` limpo; smoke manual: criar jogo com e sem modo, abandonar
  no passo 2 (nada criado), slug duplicado (erro no passo 1).
- [x] **T7** — Ritual de aprendizado (CLAUDE.md): avaliar se houve conceito novo; specs
  marcadas e resumo ao usuário.
