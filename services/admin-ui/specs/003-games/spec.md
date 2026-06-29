# Spec: 003-games

**App:** admin-ui (front-end)
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Fecha o multi-tenant no painel (ADR-0017): o `admin-ui` ganha a **gestão de jogos** e torna os achievements **por jogo**. Consome o `admin-bff/004` (`/api/games`, `/api/games/{gameId}/achievements`, `/api/games/{gameId}/players/{playerID}/achievements`). Também **conserta** a busca de achievements, que apontava para a rota antiga (sem jogo) e ficou quebrada. SDD adaptado (spec = telas/UX), como as 001/002.

## Telas e navegação

- Nova seção **"Games"** na sidebar, logo abaixo de "Visão geral" (o jogo é a dimensão de tenant, transversal aos domínios).
- A seção Games é o **hub**: registrar jogo, listar jogos, e **selecionar** um jogo ativo para ver/gerir seu catálogo.
- O **jogo selecionado** é estado compartilhado do app: o painel de achievements (domínio Achievements) usa o mesmo jogo.

## Comportamento (UX)

### Seção Games
- WHEN a seção abre -> `GET /api/games` e lista os jogos (slug, nome, criado em).
- **Registrar jogo:** formulário (slug, nome) -> `POST /api/games`. IF `409` -> "slug já existe"; IF `400` -> "slug inválido (minúsculas, sem espaço)"; no sucesso, o jogo entra na lista e é **selecionado**.
- THE APP SHALL destacar o jogo selecionado; clicar num jogo o seleciona.
- WHEN há jogo selecionado -> mostra o **catálogo** dele (`GET /api/games/{id}/achievements`) e um formulário **"adicionar conquista"** (id, nome, descrição, eventType, target) -> `POST /api/games/{id}/achievements`. IF `409` -> "id já existe neste jogo"; IF `400` -> "dados inválidos"; no sucesso, a conquista entra no catálogo.
- IF o game-registry/achievements estiver indisponível (`502`) -> estado de erro no painel, sem quebrar.

### Domínio Achievements (página)
- Mantém saúde + RED (inalterado).
- A **busca de achievements de um jogador** passa a ser **por jogo**: usa o jogo selecionado (ou um seletor local) + `playerID` -> `GET /api/games/{gameId}/players/{playerID}/achievements`.
- IF nenhum jogo selecionado -> orienta a escolher um jogo (na seção Games).
- Remove a chamada antiga `/api/players/{playerID}/achievements` (não existe mais).

### Estados comuns
- Loading na primeira carga; erro com "tentar de novo"; mensagens de validação (400/409) vindas do BFF exibidas no formulário.

## O que consome (admin-bff/004)

| Chamada | Uso |
|---|---|
| `GET /api/games` | lista de jogos |
| `POST /api/games` | registrar jogo (repassa 400/409) |
| `GET /api/games/{gameId}/achievements` | catálogo do jogo |
| `POST /api/games/{gameId}/achievements` | cadastrar conquista (repassa 400/409) |
| `GET /api/games/{gameId}/players/{playerID}/achievements` | achievements do jogador no jogo |

## Layout e visual

- Seção Games: lista de jogos (cards ou tabela, selecionável) + form de registro; abaixo, o catálogo do jogo selecionado + form de conquista. Tema dark, consistente com o resto.
- Seletor de jogo reutilizável (dropdown ou a própria lista) que define o jogo ativo compartilhado.

## Critérios de aceitação

- [ ] Seção "Games" na sidebar; lista os jogos; registrar jogo (sucesso entra/seleciona; 409/400 mostram mensagem)
- [ ] Jogo selecionado mostra o catálogo; adicionar conquista (sucesso entra; 409/400 mostram mensagem)
- [ ] Domínio Achievements: busca por jogador agora usa o jogo selecionado + playerID; sem jogo -> orienta escolher
- [ ] A rota antiga `/api/players/{id}/achievements` não é mais chamada
- [ ] 502 -> estado de erro sem quebrar; loading/empty tratados

## Fora do escopo

- **Editar/remover** jogo ou conquista — incremental
- **Auth** — interno/local no MVP
- **`game_id` nos demais domínios** (leaderboard, player-data, cloud-save) — features futuras
- **Validação rica de eventType** (lista de tipos conhecidos) — texto livre no MVP
