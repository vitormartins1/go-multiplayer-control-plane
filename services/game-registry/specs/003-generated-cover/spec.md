# Spec: 003-generated-cover

**Serviço:** game-registry (metadados) + admin-ui (renderização)
**Status:** Done (2026-06-30)
**Data:** 2026-06-30

## Contexto

Toda capa de jogo é uma **imagem determinística por seed** (default = slug), em vez do gradiente+inicial atual (admin-ui/005) ou de uma imagem default única. Sem upload no MVP. **Abordagem escolhida: gerar no cliente** a partir da seed (ver plan.md — prós/contras); o game-registry guarda só **metadados**, deixando espaço para upload futuro e troca de gerador/estilo.

## Comportamento

### game-registry
- WHEN cadastra um jogo sem imagem informada → associa metadados de imagem gerada: `imageSource="generated"`, `generatedImageSeed=slug`, `generatedImageStyle="shapes"` (default), `image=""` (reservado p/ upload).
- THE API SHALL expor esses campos em `GET /games`, `GET /games/{id}`, `POST /games`.
- Registros antigos: a migration faz backfill (`generatedImageSeed=slug`, defaults nos demais); leituras usam `COALESCE` (NULL → slug/"") para nunca quebrar.

### admin-ui
- WHEN exibe um jogo (biblioteca, header, detalhe) → renderiza a capa determinística a partir de `generatedImageSeed` (cai no `slug` se vazio) e `generatedImageStyle` (DiceBear; abstrato/geométrico, não-avatar).
- IF `imageSource="upload"` e `image` presente (futuro) → usa `image`; senão gera. (Hoje sempre gera.)
- Mesmo slug → mesma capa; slugs diferentes → capas diferentes (DiceBear é determinístico por seed).

## Contratos

- DB (migration 004): `image TEXT`, `image_source TEXT NOT NULL DEFAULT 'generated'`, `generated_image_seed TEXT`, `generated_image_style TEXT NOT NULL DEFAULT 'shapes'`.
- domain `Game` + DTO (game-registry e admin-bff): `image, imageSource, generatedImageSeed, generatedImageStyle`.
- admin-ui: `Game` ganha os campos; `GameCover` renderiza via `@dicebear/core` + `@dicebear/collection` (`coverSeed(game)=generatedImageSeed||slug`).

## Casos de Teste Obrigatórios
- [ ] (service) cadastrar jogo associa imagem gerada (imageSource=generated, seed=slug, style=default)
- [ ] (service) determinismo: mesmo slug → mesma seed; slugs diferentes → seeds diferentes
- [ ] (repo, integração) round-trip dos campos; registro com seed NULL (antigo) lê via COALESCE → slug (não quebra)
- [ ] (transport) DTO expõe os 4 campos
- [ ] (ui) build ok; capa renderiza nos cards/header/detalhe

## Fora do escopo
- **Upload** de imagem (só deixa o espaço: `image`/`imageSource`)
- Gerar/servir a imagem no backend (decisão: cliente — ver plan.md)
- Editar estilo pelo painel
