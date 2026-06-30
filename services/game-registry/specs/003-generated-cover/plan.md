# Plan: 003-generated-cover
**Spec:** specs/003-generated-cover/spec.md
**Data:** 2026-06-30

## Decisão: gerar no cliente (não no backend)
Prós/contras avaliados:
- **Backend gera+salva**: fonte única, serve qualquer cliente; MAS não há DiceBear em Go (gerador geométrico bonito = muito trabalho), e trocar estilo exige re-gerar registros.
- **Cliente gera por seed (escolhida)**: estilo bonito/abstrato é trivial em JS (DiceBear, a referência citada); trocar gerador/estilo = só atualizar o front; banco guarda só metadados (barato). Contra: outros clientes (Unity) reimplementariam — mas hoje o único cliente com capa é o painel; quando o Unity precisar, os metadados já habilitam gerar/servir no backend.
- **Espaço p/ upload**: o campo `image` + `imageSource` ficam reservados (NULL/"generated" agora); no futuro `imageSource="upload"` + `image=URL`.

Decisão localizada e reversível (metadados deixam o caminho aberto) → documentada aqui, sem ADR. (Pode virar ADR se a geração migrar pro backend.)

## Arquivos
### game-registry
- `migrations/004_add_cover_metadata.sql` — 4 colunas + backfill `generated_image_seed=slug`.
- `internal/domain/game.go` — `Game` ganha `Image, ImageSource, GeneratedImageSeed, GeneratedImageStyle`.
- `internal/repository/postgres_game.go` — Create insere os 4; List/FindByID `SELECT` com `COALESCE(image,'')` e `COALESCE(generated_image_seed, slug)` (robusto p/ antigos).
- `internal/service/game_service.go` — `Register` seta defaults (`imageSource="generated"`, seed=slug, style=`defaultImageStyle`).
- `internal/transport/http_handler.go` — `gameDTO` + `toDTO` expõem os 4 campos (json: image, imageSource, generatedImageSeed, generatedImageStyle).
- testes: service (defaults + determinismo), repo integração (round-trip + seed NULL→slug), transport (DTO).
- `infra/docker/docker-compose.yml` — migration 004 no volume (lembrar `rm -v` do game-registry-postgres).

### admin-bff (repasse)
- `internal/domain/admin.go` — `Game` ganha os 4 campos (json tags) → fluem por `/api/games` sem mais mudanças.

### admin-ui
- `package.json` — deps `@dicebear/core`, `@dicebear/collection`.
- `src/lib/api.ts` — `Game` ganha os 4 campos (opcionais).
- `src/components/GameCover.tsx` — gera SVG DiceBear via `createAvatar(style, {seed})` → data-URI; `coverSeed(game)=generatedImageSeed||slug`; estilo `generatedImageStyle||"shapes"` mapeado p/ coleção (fallback shapes); mantém faixa com nome (cover) e variante mini. `useMemo` p/ não regenerar.

## Ordem
1. game-registry (migration→domain→repo→service→transport) + testes; compose
2. admin-bff domain.Game (repasse)
3. admin-ui (deps→api→GameCover)
4. build/test/vet; npm run build; Docker E2E (cadastrar 2 jogos → capas distintas; mesmo slug → mesma capa)

## Riscos
- **Migration em volume existente** → `rm -v` do game-registry-postgres (perde jogos de teste; sandbox re-seeda).
- **Bundle**: DiceBear aumenta o JS; importar só os estilos usados (tree-shake).
- **Determinismo no front**: garantido pelo DiceBear (seed estável = slug); testes de seed ficam no backend.
