# Spec: 005-game-rename

**Serviço:** game-registry
**Status:** Draft
**Data:** 2026-07-04
**Parte de:** Console — Configurações do jogo (pares: `admin-bff/010-game-rename`, `admin-ui/012-game-settings`)

## Contexto

O game-registry só cria/lê jogos — não há como **renomear**. A aba Configurações do painel (hoje ComingSoon) precisa disso. Rename = **só o `name`**; o `slug` e o `id` são as chaves estáveis (id é a dimensão multi-tenant, ADR-0017) e **não mudam**.

## Comportamento

- WHEN `PUT /games/{id}` com `{name}` -> THE SYSTEM SHALL validar o nome (não vazio, ≤ `maxNameLen`) e atualizar só o `name` do jogo; devolve o jogo atualizado.
- IF nome inválido -> `ErrInvalidName` (400). IF o jogo não existir -> `ErrGameNotFound` (404).

## Contratos de Interface

- **Domain:** `GameService.Rename(ctx, id, name) (*Game, error)`; `GameRepository.UpdateName(ctx, id, name) error` (ErrGameNotFound se 0 linhas).
- **Service:** `Rename` — `TrimSpace` + valida (reusa `maxNameLen`), delega ao repo, relê/retorna o jogo.
- **Repository:** `UPDATE games SET name=$2 WHERE id=$1` (RowsAffected==0 → ErrGameNotFound).
- **Transport:** `PUT /games/{id}` (decode `{name}` → `Rename` → 200 DTO | erro de domínio).

## Abordagem (plan folded)

Espelha o `Register`/`create` existentes. Sem migration (só UPDATE). Testes: service (valida/delega), repo (integração: atualiza/NotFound), transport (200/400/404).

## Casos de Teste Obrigatórios

- [ ] `Rename` válido -> atualiza name, mantém slug/id
- [ ] nome vazio/grande -> `ErrInvalidName`
- [ ] id inexistente -> `ErrGameNotFound`
- [ ] `PUT /games/{id}` -> 200 com o jogo; 400 em nome inválido; 404 em inexistente

## Fora do escopo

- **Delete de jogo** — adiado (cascata cross-service, ADR futuro).
- Mudar slug/capa (a capa é derivada do slug; mudar slug quebraria a seed e o multi-tenant).
