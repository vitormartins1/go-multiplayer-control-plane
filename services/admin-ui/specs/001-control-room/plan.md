# Plan: 001-control-room
**Spec:** specs/001-control-room/spec.md
**Data:** 2026-06-29

## Abordagem técnica

SPA **React + Vite + TypeScript** sob `services/admin-ui/`. Front de apresentação (fora do hexagonal Go): consome a API do `admin-bff` via **proxy do Vite** em dev (`/api` -> `http://localhost:8088`), evitando CORS sem tocar no backend. Estado leve (sem Redux): hooks com `fetch` + um hook de **polling** para o auto-refresh. Topologia com **React Flow** (`@xyflow/react`). Tema **dark** com CSS próprio (sem lib de componentes pesada no MVP).

Navegação por **domínio**: roteamento simples (estado de seção ativa; `react-router` opcional — no MVP um router leve basta) com a sidebar fixa. Cada página de domínio é uma instância de um componente genérico `DomainPage` que consome `GET /api/services/{id}/metrics` (saúde + RED + runtime) + um slot para cards específicos (matchmaking/sessions) e o bloco "Iteração 2".

> **Depende de:** `admin-bff/002-service-metrics` (o endpoint `/api/services/{id}/metrics`). As demais chamadas (topologia, live, achievements) já existem.

## Estrutura de arquivos (scaffold)

```
services/admin-ui/
  package.json, tsconfig.json, tsconfig.node.json, vite.config.ts, index.html
  .gitignore, .env.example, README.md
  src/
    main.tsx, App.tsx
    index.css                 # tema dark (variáveis CSS de status)
    lib/
      api.ts                  # fetch client + tipos espelhando o BFF
      useLive.ts              # hook de polling (toggle + intervalo) com atualização suave
      domains.ts              # catálogo de domínios (id, label, counters, "iter2", planned)
    components/
      Sidebar.tsx             # navegação por domínio + seção ativa
      RefreshControls.tsx     # toggle auto-refresh + seletor 5/15/30s
      Overview.tsx            # topologia (React Flow) + contadores globais + banner degradado
      TopologyGraph.tsx       # React Flow (nós coloridos por status, read-only, click->domínio)
      DomainPage.tsx          # saúde + RED + cards específicos + bloco "Iteração 2"
      AchievementsPanel.tsx   # busca por playerID + lista de progresso/estado
      MetricCard.tsx, StatusBadge.tsx, ErrorState.tsx, Loading.tsx
```

## Decisões técnicas e trade-offs

- **Vite + React 18 + TS** (decisão ADR-0016): build rápido, dev server com proxy. Node 18 presente -> Vite 5.
- **Proxy do Vite em vez de CORS no BFF**: resolve a origem cruzada em dev sem alterar o backend (o risco de CORS que o `admin-bff/001` anotou). Em prod, servir o estático atrás do mesmo host/gateway (fora do MVP).
- **React Flow (`@xyflow/react`)** para a topologia: nós/arestas prontos, pan/zoom, read-only (`nodesDraggable={false}`, sem handles de conexão). Layout com posições estáticas por id (8 nós conhecidos) — simples e estável; auto-layout seria overkill.
- **Sem state lib (Redux/Zustand)**: o estado é local (seção ativa + dados por seção). Hooks `useLive` encapsulam fetch+polling. Menos peças.
- **Polling com atualização suave**: o hook mantém o último dado bom e só troca quando chega o novo; erro pontual não limpa a tela (decisão da spec). `setInterval` com cleanup e `AbortController` por ciclo.
- **`DomainPage` genérico dirigido por dados** (`domains.ts`): cada domínio declara seu `id`, label, counters específicos e itens de "iteração 2". Adicionar/ajustar domínio é editar dados, não criar telas — escala para a iteração 2.
- **CSS próprio (tema dark)**: variáveis CSS para as cores de status reutilizadas no grafo e nos badges; evita dependência de UI kit no MVP (pode entrar depois).
- **Sem `react-router` obrigatório**: navegação por estado simples; se preferir URLs por seção, `react-router-dom` é troca pequena (anotado como opção).

## Dependências e libs
- runtime: `react`, `react-dom`, `@xyflow/react`
- dev: `vite`, `@vitejs/plugin-react`, `typescript`, `@types/react`, `@types/react-dom`
- (opcional) `react-router-dom` se quisermos rota por seção

## Ordem de implementação
1. Scaffold (package.json, tsconfig, vite.config com proxy, index.html, main/App, index.css dark)
2. `lib/api.ts` (tipos + chamadas) e `lib/useLive.ts` (polling)
3. `Sidebar` + `RefreshControls` + layout base (App)
4. `Overview` + `TopologyGraph` (React Flow, cores por status, banner degradado, click->domínio)
5. `DomainPage` genérico + `domains.ts` (RED/runtime, cards específicos, bloco "Iteração 2")
6. `AchievementsPanel` (busca por playerID)
7. Estados `Loading`/`ErrorState`; "em breve" para Game analytics
8. `README.md` (como rodar: `npm install`/`npm run dev`, `.env`); verificação local com o BFF no ar

## Riscos e pontos de atenção
- **Node 18**: fixar Vite 5 e versões compatíveis (Vite 6/7 podem exigir Node 20). Travar versões no `package.json`.
- **`npm install` no ambiente**: pode exigir rede; se indisponível, o scaffold fica pronto e a verificação de build roda quando houver acesso.
- **Dependência do `admin-bff/002`**: as páginas de domínio só têm conteúdo real após o 002. Implementar o 002 antes (ordem combinada).
- **React Flow read-only**: garantir desabilitar drag/edição e que o clique no nó navegue (sem mover).
- **Atualização suave**: cuidar para o polling não causar flicker (não desmontar o grafo a cada ciclo; atualizar só os dados dos nós).
- **`playerID` é o id da conta (sub)**, não o handle — deixar claro na UI (placeholder/hint).
