# Spec: 014-game-slug-rename

**Serviço:** game-registry
**Status:** Done (spec retroativa — o código entrou em `7f174a4`, 2026-08-24, sem spec; registrada na revisão 2026-09)
**Nível:** fatia pequena (um endpoint no padrão dos vizinhos `/fleet` e `/description`)

## Contexto

A spec 005 tratava o `slug` como estável por design. O rename de produto do Capoeira Souls para **Folclore Souls**
(o `gameId`/UUID nunca mudou) mostrou um caso legítimo de troca de slug: o rótulo de painel/registro precisa
acompanhar o nome do produto sem mudar a identidade que os clients usam.

## Comportamento

- WHEN `PUT /games/{id}/slug` com `{"slug": "<novo>"}` -> THE SYSTEM SHALL validar o formato com a mesma régua do
  registro (`^[a-z0-9][a-z0-9-]{0,38}[a-z0-9]$`), gravar e devolver 200 com o jogo atualizado.
- IF o formato é inválido -> 400 `invalid_slug`.
- IF outro jogo já usa o slug (unicidade case-insensitive do banco) -> 409 `slug_taken`.
- IF o jogo não existe ou está soft-deletado -> 404.
- A `generatedImageSeed` **não muda** junto (a capa gerada do jogo continua a mesma).
- A rota está sob a internal key (`/games`, ADR-0018), como as demais do game-registry.

## Critérios de aceite

1. Os casos acima cobertos por teste de service e de handler (8 testes entregues em `7f174a4`).
2. README do serviço lista a rota e deixa claro que o `id` é estável e o `slug` pode mudar.
