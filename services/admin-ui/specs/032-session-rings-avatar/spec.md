# Spec: 032-session-rings-avatar

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** lote console-avatars.

## Contexto

As listagens de sessão (Rooms global e aba Sessions do jogo) identificam a sessão só por
`matchId`/ids truncados. Um avatar determinístico por sessão (template **Rings** do
DiceBear, seed = `sessionId`) dá uma âncora visual barata — mesma mecânica dos avatares
de jogador (024) e da capa do jogo, sem backend.

## Comportamento

- THE SYSTEM SHALL exibir um avatar DiceBear **Rings** (24–28px, seed = `sessionId`)
  junto ao match id em cada linha nas listagens de sessão (página Rooms e aba Sessions) — na Rooms a primeira coluna é o jogo, então o avatar ancora a coluna Match.
- THE SYSTEM SHALL usar o mesmo avatar (tamanho maior) no painel expandido da sessão.
- Determinístico e client-side: mesma sessão → mesmo desenho, sem fetch novo.

## Contratos de Interface (cliente)

- `components/SessionAvatar.tsx` (novo): mesmo padrão do `PlayerAvatar` (`new Style` +
  `new Avatar(...).toDataUri()`), importando `@dicebear/styles/rings.json`; sem paleta de
  fundo (o Rings já tem identidade própria sobre tema escuro — a validar visualmente).
- `Rooms.tsx` e `GameSessions.tsx`: avatar na célula do match + no painel expandido (40px).

## Casos de Teste / Verificação

- [x] cada sessão tem um desenho estável e distinto nas duas listagens
- [x] linha continua alinhada (avatar não desloca badges/colunas)
- [x] `npm run build` limpo

## Fora do escopo

- Avatar de sessão em Analytics; agrupamento visual por servidor; persistir imagem.
