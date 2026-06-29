# Spec: 001-control-room

**App:** admin-ui (front-end)
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Front-end do painel de administração ("sala de controle" — ADR-0016): uma SPA (React + Vite + TS) que consome a API do `admin-bff` e a apresenta para um operador. **Read-only** no MVP.

O painel é **organizado por domínio** (espelhando os serviços Go), não por tipo de dado. Cada serviço tem sua página; uma **Visão geral** mostra como os domínios se comunicam. As páginas de domínio são alimentadas pelo Prometheus (saúde + RED + contadores) **agora**, e os dados profundos de cada domínio entram marcados como **iteração 2**.

> **Depende de:** `admin-bff/002-service-metrics` (endpoint `GET /api/services/{id}/metrics` com saúde + RED por serviço). Topologia, contadores globais e busca de achievements já existem (`admin-bff/001`, `achievements/003`).

> **Nota de metodologia:** o ADR-0016 colocou o `admin-ui` *fora* do ciclo SDD (doc leve). Estamos estendendo o SDD a este app de apresentação de forma **adaptada** (spec = telas/UX, não Ports/hexagonal). Registrar o ajuste no ADR-0016 ao fim (adendo).

## Telas e navegação

- **Sidebar fixa à esquerda** organizada por domínio; conteúdo à direita. Tema **dark "control room"** (acentos de status verde/vermelho/cinza).
- Seções, nesta ordem:
  1. **Visão geral** — topologia (domínios que se comunicam) + contadores globais
  2. **User base** (identity)
  3. **Achievements** (achievements)
  4. **Matchmaking** (matchmaking)
  5. **Sessions** (session)
  6. **Player data** (player-data)
  7. **Cloud save** (cloud-save)
  8. **Leaderboard** (leaderboard)
  9. **Game analytics** — *em breve* (serviço ainda não existe)
- THE APP SHALL destacar a seção ativa e indicar visualmente as seções "em breve" (desabilitada/badge).

## Comportamento (UX)

### Visão geral (topologia + global)
- WHEN abre -> `GET /api/topology` desenha um **grafo (React Flow)**: nó por serviço, aresta por dependência conhecida (com label).
- THE APP SHALL colorir o nó por status: **verde** (`up=true`), **vermelho** (`up=false`), **cinza** (`up=null`).
- WHEN `sourceAvailable=false` -> **banner** ("Prometheus indisponível — status desconhecido") + nós cinza.
- THE APP SHALL mostrar os contadores globais (`GET /api/metrics/live`): fila do matchmaking, sessions ativas, matches ok/falhos; valor `null` -> "—".
- O grafo é somente leitura (pan/zoom, sem edição). WHEN um nó é clicado -> navega para a página daquele domínio.

### Página de domínio (padrão para todos os serviços)
- WHEN uma seção de domínio abre -> `GET /api/services/{id}/metrics` e mostra um **cabeçalho de saúde** (status up/down) + **RED**: taxa de requisições, taxa de erro, latência (p95).
- IF o serviço expõe **contadores específicos** -> mostrá-los como cards no topo da página:
  - **Matchmaking:** fila, matches formados, matches falhos
  - **Sessions:** sessions ativas
  - (demais domínios: só saúde + RED no MVP)
- THE APP SHALL exibir um bloco **"Iteração 2"** listando o dado profundo ainda não disponível daquele domínio (placeholder claro, não escondido):
  - **User base:** lista/contagem de usuários
  - **Achievements:** analytics agregada (taxa de unlock por conquista)
  - **Matchmaking:** salas/tickets ao vivo
  - **Sessions:** sessions ao vivo detalhadas
  - **Player data / Cloud save / Leaderboard:** dados por jogador / saves / top-N
- IF o `GET /api/services/{id}/metrics` responder erro de fonte (Prometheus fora) -> mostrar estado degradado ("métricas indisponíveis"), sem quebrar a página.

### Achievements (dado real além do RED)
- Além do padrão de domínio, a página Achievements tem **busca por jogador**: WHEN o operador informa um `playerID` e busca -> `GET /api/players/{playerID}/achievements` lista as conquistas com **progresso `progress/target`** e **estado** (desbloqueada/bloqueada; data quando desbloqueada).
- IF a lista vier vazia -> "Nenhum progresso para este jogador" (não é erro). IF responder 502 -> erro ("achievements indisponível").
- THE APP SHALL deixar claro que o `playerID` é o id da conta (sub), não o handle.

### Game analytics
- THE APP SHALL exibir a seção como **"em breve"** (o serviço de analytics ainda não existe), sem chamadas.

### Atualização dos dados ao vivo
- THE APP SHALL ter um **toggle de auto-refresh** + **seletor de intervalo** (5s / 15s / 30s), aplicado aos dados ao vivo (Visão geral e as métricas das páginas de domínio). Default: **ligado, 5s**.
- WHILE ligado -> re-buscar a seção ativa a cada intervalo, com **atualização suave** (sem piscar; mantém o render anterior em erro pontual).
- A **busca de achievements** é sob demanda (fora do auto-refresh).

### Estados comuns
- THE APP SHALL mostrar **carregando** na primeira carga de cada seção e um estado de **erro com "tentar de novo"** se o BFF estiver fora — sem quebrar o app.

## O que consome (API do admin-bff, via proxy do Vite `/api` -> `:8088`)

| Chamada | Uso | Origem |
|---|---|---|
| `GET /api/topology` | grafo de serviços + saúde | bff/001 |
| `GET /api/metrics/live` | contadores globais (Visão geral) | bff/001 |
| `GET /api/services/{id}/metrics` | saúde + RED por domínio | **bff/002 (dependência)** |
| `GET /api/players/{playerID}/achievements` | busca de achievements | bff/001 + achievements/003 |

## Layout e visual

- Sidebar escura por domínio; conteúdo com header (nome do domínio + controles de auto-refresh).
- Paleta dark; cores de status (verde/vermelho/cinza) reutilizadas no grafo e nos cabeçalhos de saúde.
- Desktop-first (uso interno em tela grande).

## Critérios de aceitação

- [ ] Sidebar por domínio (Visão geral + 7 serviços + Game analytics "em breve"); seção ativa destacada; tema dark
- [ ] Visão geral: grafo com cores por status, pan/zoom, não editável; `sourceAvailable=false` -> banner + cinza; contadores globais com `null` -> "—"; clicar no nó navega ao domínio
- [ ] Página de domínio: saúde + RED (rate/erros/latência p95) de `/api/services/{id}/metrics`; contadores específicos em matchmaking/sessions; bloco "Iteração 2" visível
- [ ] Achievements: busca por `playerID` lista progresso/estado; vazio -> mensagem; 502 -> erro
- [ ] Game analytics: seção "em breve", sem chamadas
- [ ] Toggle auto-refresh + intervalo (5/15/30s) nos dados ao vivo; default on @ 5s; atualização suave
- [ ] BFF fora -> estados de erro com "tentar de novo", sem quebrar o app

## Fora do escopo

- **Auth / login** (BFF interno/local no MVP)
- **Ações administrativas** (banir, resetar, forçar fim de sessão) — read-only
- **Dados profundos por domínio** (lista de usuários, analytics agregada, salas/sessions ao vivo, top-N, saves) — iteração 2 (precisam de endpoints novos nos serviços)
- **Serviço de game analytics** — não existe ainda (seção "em breve")
- **Responsividade mobile** e **i18n** — desktop-first, pt-BR
- **Testes e2e/visual** pesados — cobertura leve (a definir no plan)
