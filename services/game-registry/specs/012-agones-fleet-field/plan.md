# Plan 012 — campo `Fleet` por jogo

## Domain (`internal/domain/game.go`)

- `Game` (linha ~20-36) ganha `Fleet string`, irmão de `MultiplayerModes` — comentário citando ADR-0026 e o amendment (Fleet é por jogo, não por modo, porque todos os modos do mesmo jogo compartilham o mesmo binário, ADR-0024).
- `GameService` (interface, ~linha 187) ganha `SetFleet(ctx context.Context, id, fleet string) (*Game, error)` — mesmo padrão de `Rename`: valida, delega ao repo, recarrega.
- `GameRepository` ganha `UpdateFleet(ctx context.Context, id, fleet string) error` — mesmo padrão de `UpdateName`.
- `internal/domain/errors.go`: novo `ErrInvalidFleet = errors.New("game-registry: fleet inválida")`.

## Service (`internal/service/game_service.go`)

- `fleetRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)` (mesmo formato de `modeIDRE` em profile_service.go) — módulo-level var, igual ao padrão já usado ali.
- `SetFleet`: trim não se aplica (fleet não tem espaços internos por definição do regex); string vazia é válida (limpa o campo, item 3 da spec); não-vazia precisa casar `fleetRE` senão `ErrInvalidFleet`. Delega a `s.repo.UpdateFleet` e recarrega via `FindByID`, espelhando `Rename` (game_service.go:64-77).
- `GetMode` (profile_service.go:133) muda de assinatura: `GetMode(ctx, gameID, modeID) (*domain.MultiplayerMode, string, error)` — o segundo retorno é `g.Fleet`, já disponível porque `GetMode` já chama `s.repo.FindByID(ctx, gameID)` (nenhuma query extra). Único caller real é o handler de transport (mais a interface e o mock de teste).

## Repository (`internal/repository/postgres_game.go`)

- `UpdateFleet`: mesmo shape de `UpdateName` (linha 44-53) — `UPDATE games SET fleet = $2 WHERE id = $1 AND deleted_at IS NULL`, `RowsAffected() == 0` → `ErrGameNotFound`.
- `selectCols`/`scanGame`/`FindByID`: adicionar `COALESCE(fleet, '')` na lista de colunas e o scan correspondente em `g.Fleet` (mesmo tratamento NULL→"" que `image`/`generated_image_style` já recebem, linha 69-73).
- `Create`: **não** precisa inserir `fleet` no INSERT (spec não pede — jogo nasce sem Fleet, setada depois via `PUT /games/{id}/fleet`); a coluna tem default `''`/NULL.

## Migration

- `migrations/0NN_add_games_fleet.sql` (próximo número livre): `ALTER TABLE games ADD COLUMN fleet TEXT;` — nullable, sem default explícito (repo já trata NULL→"" no SELECT). **Adicionar ao mount do `docker-compose.yml`** (gotcha já mordeu 2x nesta sessão — migrations novas só rodam em volume fresco) e aplicar manualmente via `docker exec -i <postgres> psql` se o volume local já existir.

## Transport (`internal/transport/http_handler.go`)

- Nova rota: `mux.HandleFunc("PUT /games/{id}/fleet", h.setFleet)` (Register, linha ~46, ao lado de `rename`).
- `setFleet` handler: decode `{"fleet": string}`, chama `h.games.SetFleet`, `writeDomainError` em erro (novo case `ErrInvalidFleet` → `400`, mesmo tratamento de `ErrInvalidName`), `writeJSON(200, toDTO(g))` em sucesso — espelha `rename` (linha 412-426).
- `gameDTO`/`toDTO`: adicionar `Fleet string \`json:"fleet"\`` (sem `omitempty` — item 6 da spec exige `""` explícito, não omissão, mesmo raciocínio do GO-LEARNINGS #37 sobre `omitempty` não omitir strings vazias de qualquer forma — aqui é []explícito por design, não por bug).
- `modeDTO` (linha 152-156): adicionar `Fleet string \`json:"fleet,omitempty"\`` fora do embed `profileDTO` (é atributo do jogo, não do perfil).
- `toModeDTO` muda para `toModeDTO(m *domain.MultiplayerMode, fleet string) modeDTO` — os 2 outros call-sites (`listModes` linha 349, `setMode` linha 366) passam `""` ou o fleet já carregado se disponível; **`listModes`** já tem `g` carregado (via `s.repo.FindByID` dentro do `ListModes` do service) então pode devolver fleet ali também por consistência barata — mas só `getMode` é estritamente exigido pela spec (item 5, é o único endpoint que o `registryclient` do session-service chama).
- `getMode` handler (linha 369-376): `m, fleet, err := h.games.GetMode(...)`; `toModeDTO(m, fleet)`.

## Erros/HTTP

- `writeDomainError`: adicionar `case errors.Is(err, domain.ErrInvalidFleet): writeError(w, http.StatusBadRequest, "invalid_fleet")` no bloco existente (mesmo formato de `ErrInvalidName`).

## Testes

- `internal/service/game_service_test.go`: `TestSetFleet_Valido_Atualiza`, `TestSetFleet_Invalido_ErrInvalidFleet` (formato ruim), `TestSetFleet_VazioLimpaCampo`, `TestSetFleet_Inexistente_ErrGameNotFound` — espelhando os testes de `Rename`.
- `internal/repository/postgres_game_test.go`: round-trip `UpdateFleet` + `FindByID` devolve o valor; jogo sem fleet setada → `""`.
- `internal/transport/http_handler_test.go`: `PUT /games/{id}/fleet` feliz/inválido; `GET /games/{gameId}/multiplayer-profiles/{modeId}` inclui `fleet` na resposta.
- Atualizar `mockService` (transport tests) e qualquer outro mock de `GameService` para a nova assinatura de `GetMode` e o novo método `SetFleet`.
