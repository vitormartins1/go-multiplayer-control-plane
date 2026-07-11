# Spec 022 — ícone de partida a partir do snapshot do modo (+ padronização de tags)

**Serviços:** admin-bff (+ `shared/`, session-service — fatia cross-service) + admin-ui
**Status:** Draft
**Data:** 2026-07-11
**Relaciona:** session-service/017 (mode snapshot manifest, `profileHash`), admin-ui (component library — `SessionAvatar`, `GameCover`, `archetypeMeta.ts`, revisão de layout 2026-07-10 que trocou cor-por-arquétipo por ícone único)

## Contexto

Hoje o ícone DiceBear de cada partida (aba Sessions de um jogo e aba Rooms, todos os jogos) é seedado por `sessionId` — puramente decorativo, sem significado: duas sessões do MESMO modo, com a MESMA config, têm ícones diferentes só porque são sessões diferentes.

Motivação do Vitor: trocar a seed pelo **snapshot do modo** (`profileHash`, já existe e já é congelado na sessão desde a spec 017 do session-service — SHA-256 de `{archetype, topology, params}` truncado a 16 hex) — assim, o ícone passa a significar algo: sessões do mesmo modo com a mesma config mostram o MESMO ícone; alterar a config do modo (Configurações → Multiplayer) e abrir uma sessão nova muda o ícone, visível de cara na listagem, sem precisar comparar JSON.

Isso só funciona de ponta a ponta se o mesmo ícone também aparecer na aba Configurações → Multiplayer, mostrando "qual seria o ícone/hash de uma sessão aberta agora com esta config" — o que hoje **não é possível**: o hash só nasce dentro do `CreateSession`/`OpenWorld` do session-service (snapshot tirado no momento), o game-registry não computa hash nenhum. Levantamento técnico confirmou que dá pra fechar esse gap sem inventar heurística: o admin-bff já recebe do game-registry os mesmos bytes crus (`json.RawMessage`) que o session-service usa pra calcular o hash — o cálculo pode ser **reaproveitado byte-a-byte** (não reimplementado em paralelo, que arriscaria divergir).

De caminho, dois problemas de consistência visual foram achados e entram nesta spec porque tocam os mesmos componentes:
1. A "tag de modo" (badge com o nome do modo) em Rooms e Sessions ainda usa o padrão antigo (`color="indigo"` fixo) — o resto do console já migrou pra ícone-por-arquétipo + cor única (`ARCHETYPE_META`, revisão de 2026-07-10).
2. Na aba Rooms, o ícone de partida vive isolado na coluna "Match"; o Vitor quer mover pra primeira coluna, sobreposto na capa quadrada do jogo (mesmo espírito do overlap de avatares de jogador já usado em Players/Sessions).

## Comportamento (EARS)

### Seed do ícone de partida (Sessions + Rooms)

1. **O SISTEMA DEVE** trocar a seed do `SessionAvatar` de `session.sessionId` para `session.profileHash` nos 4 call sites atuais (`GameSessions.tsx` linha da tabela + painel expandido; `Rooms.tsx` linha da tabela + painel expandido).
2. **QUANDO** `profileHash` está vazio (sessão legada, anterior à spec 017), **O SISTEMA DEVE** cair de volta pra `sessionId` como seed — nunca deixar o `SessionAvatar` sem seed.
3. **O SISTEMA NÃO DEVE** persistir nem recalcular `profileHash` de sessões já existentes — é passthrough do que a sessão já carrega hoje.

### Hash do snapshot ATUAL de um modo (sem sessão aberta)

4. **O SISTEMA DEVE** expor, num pacote `shared/` novo (ex. `shared/modesnapshot`), a função de hash hoje interna ao session-service (`registryclient.profileHash`) — `Hash(archetype, topology string, params json.RawMessage) string`, mesma assinatura de bytes (SHA-256 de `archetype + 0x00 + topology + 0x00 + params`, truncado a 16 hex).
5. **session-service DEVE** passar a chamar essa função compartilhada em vez de manter a cópia local — mesmo resultado, uma fonte de verdade só.
6. **admin-bff DEVE** expor um endpoint novo que devolve o hash ATUAL de cada modo cadastrado de um jogo, calculado a partir da MESMA resposta crua que `ListMultiplayerModes` já usa (uma chamada só ao game-registry, sem N chamadas) — decodificando `{modeId, archetype, topology, params}` por item do array e aplicando a função compartilhada.
7. **QUANDO** o jogo não tem modos cadastrados, **O SISTEMA DEVE** devolver um mapa vazio (200), não erro.
8. **O SISTEMA NÃO DEVE** cachear o resultado — recalculado a cada chamada (mesmo espírito da spec 023 do session-service, volume baixo, uso interno de console).

### Aba Configurações → Multiplayer

9. **O SISTEMA DEVE** adicionar, na tabela de modos (`MultiplayerSettings.tsx`), uma coluna nova **à esquerda de tudo** (antes de "Modo") com o ícone `SessionAvatar` seedado pelo hash atual daquele modo (item 6).
10. **O SISTEMA DEVE** adicionar uma coluna com o **snapshot atual** (o hash, texto monoespaçado — mesmo padrão já usado em `SessionMatchDetail.tsx` pra `profileHash` de sessão).
11. **QUANDO** o hash de um modo ainda não carregou (chamada em andamento) ou falhou, **O SISTEMA DEVE** mostrar um estado de carregamento/vazio na célula — nunca travar a tabela inteira (os dados dos modos, já carregados por `ListMultiplayerModes`, continuam renderizando normalmente).

### Aba Rooms — primeira coluna (capa do jogo + ícone de partida sobreposto)

12. **O SISTEMA DEVE** adicionar, na primeira coluna da tabela de Rooms, a capa quadrada do jogo (`GameCover`, variante pequena já existente) com o `SessionAvatar` da partida sobreposto como badge (posição: canto inferior-direito — o canto superior-esquerdo já é usado pelo ícone de arquétipo dentro do próprio `GameCover`), com o mesmo tratamento visual de aro/borda já usado nos avatares sobrepostos de jogador (`PlayersSummary`, `box-shadow` na cor de fundo da página).
13. **O SISTEMA DEVE** remover o `SessionAvatar` da coluna "Match" (que passa a mostrar só o `matchId`) — o ícone não deve aparecer duplicado.
14. A segunda coluna ("Jogo") **CONTINUA** mostrando nome do jogo + tags de modo/arquétipo, sem mudança de conteúdo — só a capa e o ícone de partida saem dela.

### Padronização da tag de modo

15. **O SISTEMA DEVE** trocar, em `Rooms.tsx` e `GameSessions.tsx`, o badge de modo (`color="indigo"` fixo) por `color="gray"` + `leftSection` com o ícone de `ARCHETYPE_META[session.archetype]` — o texto continua sendo o `modeId` (o ícone representa a FAMÍLIA/arquétipo da simulação, não o modo individual; modos diferentes do mesmo arquétipo compartilham o mesmo ícone, de propósito).
16. **QUANDO** `session.archetype` está vazio/desconhecido (sessão legada), **O SISTEMA DEVE** manter o badge sem `leftSection` (só o texto do modo), nunca quebrar o render.

## Modelo de Dados

```go
// shared/modesnapshot
func Hash(archetype, topology string, params json.RawMessage) string
```

```
// admin-bff: GET /api/games/{gameId}/multiplayer-profiles/snapshot-hashes
// 200:
{"tdm": "a1b2c3d4e5f60718", "ranked": "9f8e7d6c5b4a3210"}
```

```ts
// admin-ui: lib/api.ts
api.modeSnapshotHashes(gameId: string, signal?: AbortSignal): Promise<Record<string, string>>
```

## Casos de Teste Obrigatórios

- [ ] `shared/modesnapshot.Hash` produz o MESMO valor que a função antiga do session-service para a mesma entrada (teste de regressão, mesmos fixtures)
- [ ] session-service: `go test ./services/session-service/...` continua verde após trocar pra função compartilhada (nenhuma sessão nova muda de hash)
- [ ] admin-bff: endpoint novo devolve o mesmo hash que uma sessão real aberta com aquela config teria (`profileHash` da sessão == hash devolvido pelo endpoint pra aquele modo, config inalterada)
- [ ] admin-bff: jogo sem modos → `{}`, 200
- [ ] admin-bff: game-registry fora do ar → erro tratável (502), não derruba o admin-ui
- [ ] admin-ui: `SessionAvatar` usa `profileHash` quando presente, cai pra `sessionId` quando ausente (sessão legada) — nos 4 call sites
- [ ] admin-ui: `MultiplayerSettings.tsx` mostra ícone + hash por modo; estado de carregamento não bloqueia a tabela de modos
- [ ] admin-ui: `Rooms.tsx` mostra capa + ícone sobreposto na 1ª coluna; coluna "Match" sem o ícone (só `matchId`); coluna "Jogo" inalterada
- [ ] admin-ui: tag de modo em Rooms/Sessions usa ícone de arquétipo + cor cinza; sessão sem `archetype` não quebra (sem ícone, só texto)
- [ ] `go test ./services/{session-service,admin-bff}/... -race` + `tsc --noEmit` + `npm run build` limpos
- [ ] Smoke manual: mudar a config de um modo em Configurações, abrir uma sessão nova daquele modo → ícone em Rooms/Sessions muda; ícone em Configurações pra aquele modo já mudou ANTES de abrir a sessão (é o "preview" do que a próxima sessão vai carregar)

## Fora de escopo

- **Adicionar badge de modo em `MultiplayerSettings.tsx`** (hoje é só `<Code>{modeId}</Code>`, sem badge nenhum) — o pedido ali foi ícone-do-snapshot + coluna de hash, não uma tag de modo nova. Se fizer sentido depois, é aditivo, não parte desta spec.
- **Persistir/versionar snapshots históricos de um modo** (ex. "ver o hash de como o modo estava há 3 dias") — só o hash ATUAL é exposto; sessões antigas já carregam o hash congelado delas mesmas (`profileHash` da sessão), isso não muda.
- **Cache do endpoint de hashes** — sem necessidade hoje (volume baixo); revisar se o padrão de uso mudar.
- **Aplicar o ícone de arquétipo em qualquer outro lugar fora de Rooms/Sessions** — o levantamento não achou outro `color="indigo"` de modo no admin-ui; se aparecer um novo em outra tela, é fora desta spec.
