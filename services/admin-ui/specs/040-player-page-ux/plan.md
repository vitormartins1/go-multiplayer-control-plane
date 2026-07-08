# Plan 040 — UX da página do jogador (admin-ui)

## Decisões técnicas

### Handle: cadeia identity → BFF → UI (lookup por id, não paginação)

O BFF já resolve handles em lote paginando `GET /admin/players` (`handlesByID`), mas
para UM jogador isso é O(base inteira) por visita à página. Em vez disso:

- **identity**: `FindByID` no `AccountRepository` (projeção sem `password_hash`,
  como `ListAccounts`) + `GetPlayer` na Port `PlayerLister` + rota
  `GET /admin/players/{playerID}`. `ErrAccountNotFound` passa a ter um segundo
  consumidor: na superfície admin ele vira `404 player_not_found` (no login continua
  virando `ErrInvalidCredentials` — comportamento intacto).
- **BFF**: `GetPlayer` na Port `PlayerDirectory` + método no client identity
  (404 → `domain.ErrPlayerNotFound`, resto → `ErrIdentityUnavailable`) + rota
  `GET /api/players/{playerID}`.
- **UI**: `api.player(id)` best-effort no `PlayerDetail`; `guest-` curto-circuita
  no cliente (sem fetch). Falha → header mantém "Jogador" (fallback atual).

### Seletor por capas: componente novo, estado onde já está

`GameChipsSelector` (componente novo em `components/`) recebe `games`, `value`,
`onChange` — o estado `gameId` continua no `PlayerDetail` (nada muda para as abas).
Chip = `UnstyledButton` com `GameCover variant="mini"` + nome truncado
(`maxWidth` + ellipsis); selecionado com borda `--mantine-color-blue-5` e fundo
sutil. Overflow: os 4 primeiros + o selecionado (quando fora dos 4) ficam visíveis;
"+N" abre `Menu` com os demais. Regra concreta de visibilidade:
`visible = primeiros 5` e, se `selected ∉ visible`, substitui o 5º.

### Jogo de origem: router state, não query param

`GamePlayers` navega com `nav(`/players/${id}`, { state: { fromGameId: gameId } })`.
`useLocation().state` é lido no efeito de pré-seleção do `PlayerDetail`: se o
`fromGameId` está na lista, seleciona; senão cai no primeiro. State não polui a URL
e sobrevive ao back/forward — deep-link continua caindo no default, como hoje.

### Achievements: derivação client-side, sem chamada nova

`ListForPlayer` já devolve o catálogo inteiro anotado (progresso 0 incluído).
A visão "Por estado" é um `useMemo` que particiona `data` em três grupos
(unlocked / progress>0 / progress==0). `SegmentedControl` com estado local
(`useState<"table" | "grouped">`, default `"table"`).

Barra por linha: `Progress` do Mantine com `value = min(progress/target, 1) * 100`;
verde quando unlocked. Uma célula `Group` mantém o numeral `X/Y` ao lado da barra.

Progresso geral: `useMemo` sobre `data` → `{ unlocked, total, pct }` com
`pct = Σ min(progress, target) / Σ target` (guarda contra `Σ target === 0`).
`Tooltip` na barra com a frase do cálculo. Exibido nas duas visões.

## Arquivos a modificar / criar

```
services/identity-service/
  internal/domain/account.go            ← FindByID na Port; GetPlayer no PlayerLister
  internal/domain/errors.go             ← comentário do ErrAccountNotFound (novo consumidor)
  internal/repository/postgres_account.go ← FindByID (projeção sem hash)
  internal/service/admin_list.go        ← GetPlayer (wrap de erro; not-found passa)
  internal/transport/http_handler.go    ← rota GET /admin/players/{playerID}

services/admin-bff/
  internal/domain/admin.go              ← GetPlayer na Port PlayerDirectory; ErrPlayerNotFound
  internal/identity/client.go           ← GetPlayer (404 → ErrPlayerNotFound)
  internal/service/admin_service.go     ← Player(ctx, id) delegando
  internal/transport/http_handler.go    ← rota GET /api/players/{playerID}

services/admin-ui/src/
  lib/api.ts                            ← api.player(playerID)
  components/GameChipsSelector.tsx      ← novo: fileira de chips com mini-capa + overflow
  pages/PlayerDetail.tsx                ← handle no header/breadcrumb, chips, fromGameId
  pages/GamePlayers.tsx                 ← nav com state { fromGameId }
  pages/PlayerAchievements.tsx          ← SegmentedControl, 3 tabelas, barras, geral
```

Nenhuma dependência nova (Go ou npm).

## Testes

- identity: service (`GetPlayer` ok / not-found / erro de repo) + transport
  (200 / 404) com os mocks existentes; repositório segue o padrão do arquivo.
- BFF: transport (200 / 404 / 502) com o stub de reader/directory existente.
- UI: sem test runner no projeto — verificação por `npm run build` + casos manuais
  da spec (collection .http não se aplica; é navegação).
