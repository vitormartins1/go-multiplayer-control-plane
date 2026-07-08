# Plan: 035-game-create-wizard

## Decisões técnicas

1. **Escrita só no Confirmar.** O wizard acumula estado local (`{name, slug, mode?}`)
   e executa `POST /games` → `PUT .../multiplayer-profiles/{modeId}` em sequência no
   passo 3. Alternativa rejeitada: criar o jogo ao sair do passo 1 (deixa órfãos em
   abandono e obriga rollback/limpeza que a API não tem).
2. **Reuso do ModeEditor, não um form novo.** O editor por arquétipo (MultiplayerSettings)
   passa a receber `onSubmit(modeId, displayName, profile) => Promise<void>` e um rótulo
   de botão; Configurações injeta o save de API atual (comportamento idêntico), o wizard
   injeta um coletor local. Alternativa rejeitada: form simplificado no wizard (duplicaria
   validação/params dos 4 arquétipos e divergiria com o tempo).
3. **Preview da capa com Game sintético.** `GameCover` é client-side puro sobre
   `generatedImageSeed`/`slug` — o wizard monta `{slug, name, imageSource: "generated",
   generatedImageSeed: slug, generatedImageStyle: "shapes"}` e o preview reage ao digitar.
   Usa `withName` da 036 conforme couber.
4. **Slug sugerido do nome** com slugify local (minúsculas, acentos→ascii, espaços→`-`,
   regex do backend como guia); o campo fica editável e trava depois que o usuário o
   tocar manualmente (padrão comum de "slug touched").
5. **Navegação pós-sucesso** via `useNavigate` para `/games/{id}` (a resposta do POST
   traz o id).

## Arquivos

- `src/pages/GameCreateWizard.tsx` (novo) — Stepper + estado do wizard + submissão.
- `src/pages/Games.tsx` — troca o Modal atual pela abertura do wizard.
- `src/pages/MultiplayerSettings.tsx` — `ModeEditor` exportado com `onSubmit` plugável;
  Configurações continua funcionando igual (via 037/Drawer).
- `src/lib/slug.ts` (novo, ~15 linhas) — slugify puro.

## Riscos / atenção

- `ModeEditor` hoje conhece `gameId` para o PUT — o refactor move essa dependência para
  o caller (Configurações), o wizard não tem gameId até o Confirmar.
- Validação do modeId (`DEFAULT_MODE_ID`, regex) deve continuar idêntica nos dois hosts.
- Falha parcial (jogo sim, modo não): sem rollback; notificação direciona (spec).
