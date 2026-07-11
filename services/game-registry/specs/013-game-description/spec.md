# 013 — Descrição do jogo

**Serviço:** game-registry
**Status:** Done

## Contexto

Revisão de layout do admin-ui (2026-07-11): a seção "Identidade" de
Configurações do jogo tinha pouco conteúdo (só Nome + Slug), deixando o card
com espaço vazio desproporcional depois que a capa virou um banner de largura
total. Um campo de descrição curta preenche esse espaço com conteúdo real,
além de ser um metadado razoável para qualquer registro de jogo.

Fatia pequena/aditiva (CLAUDE.md): campo novo, sem decisão técnica — segue o
mesmo padrão já usado por `Fleet` (migration 007, `SetFleet`/`UpdateFleet`).

## Comportamento

- `games.description` (`TEXT`, nullable) — texto livre, opcional, até 500
  caracteres. Vazio/NULL tratado como `""` na leitura (mesma convenção do
  `Fleet`).
- `PUT /games/{id}/description` (game-registry) — upsert do campo.
  `400 invalid_description` se passar de 500 caracteres. `404` se o jogo não
  existir (ou estiver soft-deleted).
- `GET /games` e `GET /games/{id}` passam a incluir `description` na resposta
  (mesmo carregamento do `fleet` — sem fan-out extra, é coluna simples da
  tabela `games`).
- admin-bff repassa o campo cru (passthrough, mesmo padrão do `fleet`):
  `PUT /api/games/{gameId}/description`, e `Game.Description` no DTO de
  listagem/detalhe.
- admin-ui: `Textarea` na seção Identidade (Configurações do jogo),
  autosize, salva junto do botão "Salvar" existente (mesmo clique salva nome
  e descrição, se algum dos dois mudou).

## Critérios de aceite

- [ ] Criar um jogo, setar uma descrição de 500 caracteres exatos → salva.
- [ ] Setar descrição de 501 caracteres → `400 invalid_description`, nada
      persiste.
- [ ] Descrição vazia é um estado válido (campo opcional) — jogo sem
      descrição nunca teve um erro por causa disso.
- [ ] `GET /games` (listagem) inclui `description` de cada jogo sem custo de
      fan-out adicional (mesma query que já carrega `fleet`).
- [ ] admin-ui: editar a descrição e clicar "Salvar" persiste; recarregar a
      página mostra o valor salvo.
