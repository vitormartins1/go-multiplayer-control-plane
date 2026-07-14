# Spec: 082-games-review-mode-id-and-settings

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 081 (sessão de exploração visual iterativa).

## Contexto

Três pedidos do usuário, tratados juntos por terem tocado os mesmos
arquivos (`GameCover`, `Games`, cadastro de jogo):

1. **Design review crítico da aba Games** — a biblioteca de jogos não
   passava por revisão desde o início do projeto (só ganhou os badges de
   arquétipo no card e o wizard de registro depois, spec 035/036).
2. **ID do modo travado em "default" no registro inicial** — o
   formulário já tinha o campo "ID do modo" (`ModeIdentityFields`,
   compartilhado com "adicionar modo" em Configurações), mas
   `GameCreateWizard.tsx` chamava `useModeIdentityState(null, true, [])`
   — o segundo argumento (`lockDefault`) travava o campo. A validação
   (`MODE_ID_RE`) já existia pronta, só nunca era aplicada aqui.
3. **Nova aba "Settings"** (control plane em geral, não por-jogo) — pra
   começar, trocar o estilo de capa (DiceBear) usado em Games; Players
   (registrados + guests) documentado como próxima expansão, não
   implementado agora.

## Decisões

### 1. Mode ID desbloqueado

`GameCreateWizard.tsx`: `useModeIdentityState(null, true, [])` →
`useModeIdentityState(null, false, [])`. Mudança de uma linha — todo o
resto (validação, mensagens de erro, campo `disabled`) já existia e
generaliza sozinho. Testado ao vivo: registrei um jogo com
`modeId: "ranked-duo"`, confirmei via API que persistiu (não caiu pra
"default"), removi o jogo de teste depois.

### 2. Games.tsx — design review

- Título ganha contagem (`Games · 5`), mesmo padrão de `Players.tsx`.
- Busca por nome/slug (`TextInput` + `IconSearch`), mesmo padrão de
  `Players.tsx` — só aparece com mais de 1 jogo (filtrar 1 item não
  ajuda ninguém).
- Hover do card: a mutação de `style.transform` via
  `onMouseEnter`/`onMouseLeave` (JS puro) virou uma classe CSS
  (`Games.css`, `.game-card:hover`) — mesmo efeito visual, mas resulta
  em CSS que o browser já otimiza e que respeita
  `prefers-reduced-motion` automaticamente (a versão em JS não).
- O resto (grid de capas, badges de arquétipo no canto) já estava bem
  resolvido — não mexido.

### 3. Settings (nova página, nav principal)

- Rota `/settings`, ícone `IconSettings` (mesmo ícone da aba
  "Configurações" de cada jogo — mesmo conceito, escopo diferente).
- **Preferência client-side, não campo por-jogo**: `Game.
  generatedImageStyle` existe no backend (game-registry) mas é sempre
  `"shapes"` fixo (`defaultImageStyle`, sem endpoint de update) — não
  existe hoje nenhum jeito real de setar isso diferente por jogo. A
  escolha do usuário é "configuração do control plane", não do jogo —
  por isso vira uma preferência em `localStorage`
  (`src/lib/settings.tsx`, `SettingsProvider` + `useSettings()`), lida
  por `GameCover.tsx` em vez do campo do backend. Sincroniza entre abas
  abertas (evento `storage`).
- 7 estilos (todos já instalados via `@dicebear/styles`, conferidos no
  pacote): Shapes, Stripes, Triangles, Shape Grid, Initials, Glass,
  Disco. Picker visual (preview real de cada estilo, seed fixa) em vez
  de dropdown de texto — a escolha é visual, não dá pra decidir só pelo
  nome.
- Seção "Aparência dos jogadores" (Players registrados + guests):
  existe, mesmo picker, **desabilitada** (`opacity` + `pointerEvents:
  none` + badge "em breve") — estrutura pronta pra quando isso for
  implementado de verdade (pedido explícito do usuário: "deixa a aba já
  criada, disabled por padrão").

## Critérios de Aceite

1. Registrar um jogo permite escolher o ID do modo inicial livremente
   (validado: `ranked-duo` persistiu, não caiu pra `default`).
2. Games mostra contagem no título, tem busca funcional, hover sem JS.
3. `/settings` existe no nav; trocar o estilo de capa muda todas as
   capas do console na hora (mesma aba, sem reload) e persiste depois
   de um reload completo (testado: Disco aplicado e revalidado após
   reload; localStorage por navegador/dispositivo, não por conta).
4. Seção "Aparência dos jogadores" existe, visualmente idêntica em
   estrutura, mas não interativa.
5. `tsc`/`build` limpos; sem erro de console; responsivo até 390px.

## Fora de escopo

- Persistência server-side da preferência de estilo (hoje é só
  `localStorage`, por navegador) — documentado como possível evolução
  se algum dia precisar ser por-conta/multi-dispositivo.
- Implementar de verdade a seção "Aparência dos jogadores" (players
  registrados/guests) — só a estrutura desabilitada, como pedido.
- Qualquer outro campo de Settings além do estilo de capa — a página já
  fica pronta pra crescer "com o tempo", como o usuário descreveu.
