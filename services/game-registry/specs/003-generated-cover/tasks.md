# Tasks: 003-generated-cover
**Plan:** specs/003-generated-cover/plan.md
**Data:** 2026-06-30

## game-registry
- [x] migration 004 (image, image_source, generated_image_seed, generated_image_style + backfill seed=slug)
- [x] domain Game + 4 campos
- [x] repository Create/List/FindByID (COALESCE seed→slug, image→'') + teste integração
- [x] service Register (defaults: generated/seed=slug/style) + testes (defaults + determinismo)
- [x] transport gameDTO + 4 campos + teste DTO
- [x] compose (migration 004 no volume; rm -v)

## admin-bff
- [x] domain.Game + 4 campos (repasse via /api/games)

## admin-ui
- [x] deps @dicebear/core + @dicebear/collection
- [x] api.ts Game + campos
- [x] GameCover via DiceBear (coverSeed = seed||slug; estilo mapeado; fallback; ramo p/ upload futuro)

## verificação
- [x] go test ./... -race + build/vet; npm run build
- [x] Docker E2E: 2 jogos → seeds distintas; seed=slug determinístico; sandbox antigo via COALESCE não quebra
