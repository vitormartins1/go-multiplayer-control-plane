# Spec: 040-player-page-ux

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Depende de:** admin-bff/020 (player por id) e identity/004 (lookup admin por id) — só para o item 1.

## Contexto

A página do jogador (`PlayerDetail`) hoje: header com o título fixo "Jogador" + id cru,
`Select` (dropdown) para escolher o jogo, aba Achievements com uma tabela única sem
barras de progresso, e breadcrumb `Players › <id truncado>`. Ao chegar vindo da aba
Players de um jogo (`GamePlayers`), o seletor ignora a origem e pré-seleciona sempre o
primeiro jogo da lista.

Quatro melhorias de UX, todas na mesma página (por isso uma spec só):

## Comportamento

### 1. Handle no header (e no breadcrumb)

- THE SYSTEM SHALL exibir o **handle** do jogador no lugar do título fixo "Jogador";
  o id completo permanece abaixo, dimmed (como hoje).
- THE SYSTEM SHALL exibir o handle também no breadcrumb (`Players › <handle>`), com
  fallback para o id truncado enquanto carrega ou se a resolução falhar.
- WHEN o id tiver prefixo `guest-`, THE SYSTEM SHALL exibir "Convidado" +
  `GuestBadge`, sem consultar o identity (guests não têm conta).
- IF o identity/BFF estiver indisponível ou devolver 404, THEN o header mantém o
  fallback "Jogador" (comportamento atual) — falha de resolução não bloqueia a página.

### 2. Seletor de jogo por capas

- THE SYSTEM SHALL substituir o `Select` por uma fileira de **chips com mini-capa**
  (`GameCover variant="mini"` + nome do jogo), alinhada à direita do header.
- O chip selecionado tem destaque visual (borda/preenchimento na cor primária);
  clicar em outro chip troca o `gameId` do contexto — Achievements, Player data e
  Cloud saves passam a refletir o jogo escolhido (estado já compartilhado hoje).
- O nome no chip trunca com reticências (largura máx. por chip) — não quebra layout.
- Até **5 jogos** visíveis. WHEN o jogador tiver mais de 5, THE SYSTEM SHALL exibir
  um chip "+N" que abre um `Menu` com os demais (mini-capa + nome por item).
- WHEN o jogo selecionado estiver no overflow, THE SYSTEM SHALL exibi-lo no lugar do
  5º chip visível (o selecionado nunca fica escondido).
- O default permanece: primeiro jogo da lista pré-selecionado (ou o jogo de origem,
  item 4). Com 1 jogo só, exibe-se o chip único, selecionado.

### 3. Achievements: visualização por estado + barras de progresso

- THE SYSTEM SHALL oferecer um `SegmentedControl` ("Tabela" | "Por estado") ao lado
  do botão Atualizar. Default: "Tabela" (visão atual). A escolha não persiste entre
  visitas (estado local).
- Na visão **"Por estado"**, THE SYSTEM SHALL exibir três tabelas, nesta ordem, cada
  uma com título e contagem:
  1. **Conquistadas** — `unlocked === true` (com data de desbloqueio);
  2. **Em progresso** — `!unlocked && progress > 0`;
  3. **Bloqueadas** — `!unlocked && progress === 0`.
  Grupo vazio: título com contagem `· 0` e linha dimmed "nenhuma" (não some — o
  admin vê que a categoria existe e está vazia).
- THE SYSTEM SHALL exibir, em **ambas** as visões, uma barra de progresso
  (`Progress`) na coluna Progresso, **mantendo o numeral** `X/Y` ao lado.
  Conquistada = barra 100% verde; bloqueada = barra 0% (trilho vazio).
- THE SYSTEM SHALL exibir, acima da(s) tabela(s), um **progresso geral** do jogador
  no jogo: barra + texto `X de N desbloqueadas · Y% do progresso total`.
- O percentual geral é **ponderado pelo alvo** de cada conquista:
  `Y = Σ min(progress, target) / Σ target` (unidades de progresso cumpridas sobre o
  total de unidades do catálogo). Um tooltip na barra explica o cálculo em uma frase.
  Racional: aproxima "esforço" pelo tamanho do alvo (1/3600 pesa mais que 1/3);
  distorção conhecida e aceita: alvos em unidades diferentes (segundos vs. kills)
  não são esforço comparável — por isso o numeral `X de N desbloqueadas` fica
  sempre junto, como o fato bruto.

### 4. Contexto de origem (jogo pré-selecionado)

- WHEN a navegação para `/players/{id}` partir da aba Players de um jogo
  (`GamePlayers`), THE SYSTEM SHALL levar o `gameId` de origem (router state) e
  pré-selecionar esse jogo no seletor de capas, em vez do primeiro da lista.
- IF o jogo de origem não estiver na lista do jogador (corrida/dado velho), THEN
  cai no default (primeiro da lista), sem erro.
- O breadcrumb permanece **canônico** (`Players › <handle>`): não montamos
  hierarquia falsa `Games › <jogo> › <jogador>` — breadcrumb é localização, não
  histórico; a volta ao jogo fica com o "voltar" do navegador.

## Contratos de Interface

- **Novo no BFF** (spec admin-bff/020): `GET /api/players/{playerID}` →
  `{ id, handle, createdAt }`; `404` quando não encontrado; `502` identity fora.
- **Novo no identity** (spec identity/004): `GET /admin/players/{playerID}` —
  lookup direto por id (evita o BFF paginar a lista inteira só para um handle).
- Cliente: `api.player(playerID)` novo em `lib/api.ts`; `PlayerDetail.tsx` (header,
  breadcrumb, seletor de capas, router state); `PlayerAchievements.tsx` (segmented,
  três tabelas, barras, progresso geral); `GamePlayers.tsx` (passar state no `nav`).
  Nenhuma mudança em `PlayerDataView`/`CloudSaveView` (só recebem `gameId`, como hoje).

## Casos de Teste / Verificação

- [ ] jogador registrado → header e breadcrumb com handle; id dimmed abaixo
- [ ] id `guest-…` → "Convidado" + GuestBadge, sem chamada ao endpoint
- [ ] identity fora → header "Jogador" (fallback), página funciona
- [ ] jogador com ≤5 jogos → todos os chips visíveis, primeiro selecionado
- [ ] jogador com >5 jogos → 4 visíveis + "+N"; selecionar do menu traz o jogo
      para a fileira visível
- [ ] trocar de chip atualiza Achievements, Player data e Cloud saves
- [ ] visão "Por estado" → três tabelas na ordem, contagens corretas, grupo vazio
      com "nenhuma"
- [ ] barras por linha nas duas visões, numeral X/Y preservado
- [ ] progresso geral: catálogo `target 3 (3/3) + target 3600 (0/3600)` →
      `1 de 2 desbloqueadas · ~0,1%` (3/3603)
- [ ] navegar de Games › jogo › Players → jogador abre com o jogo de origem
      selecionado
- [x] `npm run build` limpo; testes do BFF/identity dos endpoints novos

## Fora do escopo

- Breadcrumb hierárquico `Games › jogo › jogador` (anti-padrão: breadcrumb-histórico);
  handle no leaderboard (fatia própria, já anotada na 031); persistir a seleção de
  jogo entre visitas; paginação/busca no menu de overflow de jogos.
