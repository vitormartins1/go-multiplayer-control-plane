# Spec: 004-redesign

**App:** admin-ui (front-end)
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Redesign do painel para algo **profissional e focado em features de produto**, não em observabilidade (que fica no Grafana). Reorganiza a informação **por entidade administrada** (jogo, jogador) em vez de **por microserviço**, e adota uma lib de UI (**Mantine**) + **react-router** para um padrão consistente. Supersede as partes de RED/auto-refresh das specs 001/002/003.

## Arquitetura de Informação (navegação)

Sidebar com 4 seções (Mantine `AppShell`):

- **Visão geral** (`/`) — mapa arquitetural (React Flow) + **saúde leve** dos serviços (só status up/down/unknown). Sem RED/memória.
- **Games** (`/games`) — lista de jogos + **Registrar jogo** (modal). Detalhe (`/games/:gameId`) com abas:
  - **Achievements** — catálogo + **Nova conquista** (modal). *(o cadastro mora aqui)*
  - **Leaderboards**, **Configurações** — abas **"em breve"** (esqueleto).
- **Players** (`/players`) — lista de usuários (busca/paginação). Detalhe (`/players/:playerId`) com abas:
  - **Achievements** — progresso do jogador num jogo (seletor de jogo).
  - **Player data**, **Cloud saves** — abas **"em breve"**.
- **Analytics** (`/analytics`) — **"em breve"** (serviço não existe).

## Comportamento (UX)

- THE APP SHALL usar Mantine (tema dark) e react-router; navegação por URL (lista → detalhe → aba), com botão voltar funcional.
- **Sem auto-refresh:** cada tela carrega ao abrir; um botão **"Atualizar"** leve recarrega sob demanda.
- **Saúde leve:** badge de status (verde up / vermelho down / cinza desconhecido) derivado do `up` da topologia; nada de req/s, latência, memória.
- **Games:** registrar jogo (modal, valida; 409 "slug já existe", 400 "inválido"); listar; clicar abre o detalhe. Aba Achievements: catálogo + nova conquista (modal; 409/400 tratados).
- **Players:** listar (busca por id, paginação); clicar abre o detalhe; aba Achievements pede um jogo (seletor) + mostra progresso.
- **Esqueletos do futuro:** abas/seção "em breve" renderizam um placeholder claro (ícone + texto), sem chamadas.
- **Erros/vazios/loading:** estados consistentes (Mantine `Loader`, `Alert`, mensagens nos forms via `Notifications`).
- THE APP SHALL exibir feedback de sucesso (notificação) ao registrar jogo / criar conquista.

## O que consome (admin-bff)

`GET /api/topology` (mapa+saúde), `GET /api/games`, `POST /api/games`, `GET/POST /api/games/{gameId}/achievements`, `GET /api/players`, `GET /api/games/{gameId}/players/{playerID}/achievements`.

> **Não** usa mais `/api/services/{id}/metrics` (RED) nem `/api/metrics/live` — o detalhe operacional fica no Grafana. (Companion: remover o endpoint de RED do BFF — ver plan.)

## Critérios de aceitação

- [ ] Sidebar com 4 seções (Visão geral / Games / Players / Analytics); navegação por URL com voltar
- [ ] Visão geral: mapa + saúde leve (sem RED/memória)
- [ ] Games: lista + registrar (modal, 409/400); detalhe com abas; Achievements com catálogo + nova conquista
- [ ] Players: lista (busca/paginação) + detalhe; aba Achievements por jogo
- [ ] Abas/seções "em breve" como esqueleto (Leaderboards, Configurações, Player data, Cloud saves, Analytics)
- [ ] Sem auto-refresh; botão "Atualizar"; notificações de sucesso; estados de erro/vazio/loading
- [ ] Visual Mantine dark, consistente

## Fora do escopo

- Backends das abas "em breve" (leaderboard/player-data/cloud-save/analytics por jogo) — features futuras
- Editar/remover jogo ou conquista — incremental
- Auth — interno/local no MVP
