# ADR-0016: Painel admin — BFF em Go + SPA, agregação via APIs/Prometheus, sem acesso direto a bancos

**Status:** Accepted
**Data:** 2026-06-29
**Serviço(s) afetado(s):** admin-bff (novo), admin-ui (novo), infra
**Relaciona:** ADR-0002 (hexagonal), ADR-0006 (Prometheus/Grafana), ADR-0010 (control plane), ADR-0015 (database-per-service)

---

## Contexto

A stack de observabilidade (ADR-0006/0014) cobre bem o **plano operacional** — saúde, séries temporais, RED, latência, traces. Falta uma visão de **produto/administração**: navegar entidades do domínio (jogadores, achievements, leaderboards), ver o estado de runtime dos serviços de jogo (fila de matchmaking, sessions ativas) e, no futuro, executar **ações administrativas** (forçar fim de sessão, banir, resetar progresso). Isso não é função do Grafana (séries temporais e infra), e construí-lo lá seria forçar a ferramenta.

Duas restrições do projeto moldam a solução:

1. **Database-per-service (ADR-0015):** não há banco único. Cada serviço é dono exclusivo do seu schema e **nenhum serviço acessa o banco de outro** — integração só por API/evento.
2. **Estado em memória (ADR-0010):** matchmaking e session guardam fila/sessions em memória, fora de qualquer banco — só observáveis via métricas hoje.

A decisão é necessária **antes** de criar o componente, pois define a fronteira de dados, a fronteira com o Grafana e onde (ou se) Go é usado.

## Alternativas Consideradas

### Opção A: BFF em Go + SPA (React/Vite/TS), agregando APIs + Prometheus
- **Prós:** respeita database-per-service (BFF chama APIs, não bancos); um único endpoint limpo pro front; reaproveita a instrumentação Prometheus existente pros números ao vivo; o BFF é exercício idiomático de Go (fan-out concorrente com `errgroup`); front em ecossistema produtivo pra painel rico.
- **Contras:** mais um serviço Go + um app front pra manter; listas detalhadas de runtime exigirão novos endpoints admin nos serviços (trabalho diferível).

### Opção B: Front fala direto com os N serviços (sem BFF)
- **Prós:** menos uma peça.
- **Contras:** o front passa a conhecer N hosts, N contratos (REST + gRPC), CORS/credenciais por serviço; agregação e composição viram lógica no browser; acoplamento alto e frágil.

### Opção C: Painel lê os bancos direto (read-only) ou estende o Grafana (datasource SQL/plugins)
- **Prós:** caminho mais curto pra dados persistidos.
- **Contras:** **viola o ADR-0015** (acopla a 5 schemas internos); **não vê o estado em memória** (matchmaking/session), que é justamente o "sala de controle"; estender o Grafana mistura plano operacional com plano de produto e luta contra a ferramenta.

### Opção D: Go full-stack (templ + HTMX), sem SPA separada
- **Prós:** tudo em Go, zero build de JS, mais simples de operar, mais aprendizado Go.
- **Contras:** interatividade e visualização (tabelas ricas, gráficos, mapa de topologia) bem mais limitadas pro tipo de painel desejado.

## Decisão

Adotamos a **Opção A**, com os seguintes princípios:

1. **Dois componentes novos:** `services/admin-ui/` (**React + Vite + TypeScript**, SPA) e `services/admin-bff/` (**Go**, arquitetura hexagonal como os demais — ADR-0002). Go entra no **BFF**, não no front.
2. **Fronteira de dados — duas fontes, nunca banco alheio:**
   - **APIs REST/gRPC** dos serviços para entidades persistidas (identity, achievements, leaderboard, …).
   - **HTTP API do Prometheus** (`/api/v1/query`) para agregados e estado ao vivo (`matchmaking_queue_size`, `session_active_sessions`, `up{}`, etc.).
   - O BFF **nunca** lê o banco de outro serviço (respeita ADR-0015).
3. **Read-only no MVP.** Ações administrativas são evolução (iteração 2) e, quando vierem, usam **endpoints admin dedicados** nos serviços — não acesso direto a estado.
4. **Fronteira com o Grafana:** Grafana = operacional/séries/infra/alertas; admin = entidades de produto, navegação e ações. Não duplicar gráficos de infra; é aceitável **embutir** um painel do Grafana via iframe quando útil.
5. **Escopo do MVP** (valor ÷ custo, consome só o que já existe): **(a)** topologia + saúde dos serviços, **(b)** contadores ao vivo via Prometheus, **(c)** jogadores, **(d)** achievements por player. Sem novos endpoints nos serviços, exceto possivelmente um de listagem/contagem no identity.
6. **Evolução em duas iterações:** MVP consome o existente; iteração 2 adiciona listas de runtime detalhadas (ex.: `ListSessions`) e ações admin, deliberadamente.
7. **SDD:** o `admin-bff` é serviço de comportamento → segue o ciclo SDD (spec/plan/tasks). O `admin-ui` é app de apresentação → documentação leve (README), fora do ciclo SDD (alinhado ao SDD-GUIDE, que reserva SDD aos serviços Go).

## Consequências

### Positivas
- Visão de produto/admin sem poluir o Grafana; fronteiras de serviço preservadas (ADR-0015 intacto); MVP entrega valor sem tocar nos serviços Go; o BFF vira um bom exercício de concorrência em Go.

### Negativas / Trade-offs
- Mais duas peças pra manter (um serviço Go + um app front); o "ao vivo detalhado" e as ações admin ficam para a iteração 2 (no MVP, runtime é número agregado, não lista).

### Neutras / Implicações
- O `admin-bff` terá seu `plan.md` definindo compute (ADR-0005) e como consome Prometheus + APIs; não tem banco próprio no MVP.
- Pode surgir a necessidade de **service-to-service auth** entre o BFF e os serviços — mesma discussão já anotada para o fan-out do session e a integração matchmaking→session; possível ADR futuro.
- Localização sugerida: `services/admin-bff/` e `services/admin-ui/` (ou `apps/admin-console/` se preferir separar apps de serviços) — a definir no plan.

## Adendo (2026-06-29) — SDD adaptado também para o `admin-ui`

A decisão 7 (acima) colocou o `admin-ui` *fora* do ciclo SDD. Na prática, optamos por **estender o SDD ao front de forma adaptada**: o `admin-ui` ganhou `spec/plan/tasks` (`services/admin-ui/specs/001-control-room/`), mas a **spec descreve telas, funcionalidades e comportamento de UX** (EARS onde encaixa), **não** Ports/hexagonal/contratos de serviço — esses não mapeiam para um app de apresentação. O "Contratos de Interface" do template vira **"o que o app consome"** (os endpoints do BFF). Isso ajusta a decisão 7: SDD aplica-se ao front **na forma adaptada**, não como doc leve solta. O SDD-GUIDE segue reservando o template *de serviço Go* aos serviços Go.

Decorrência prática: o painel foi **reorganizado por domínio** (uma seção por serviço Go), o que exigiu uma feature extra no BFF — `admin-bff/002-service-metrics` (`GET /api/services/{id}/metrics`, saúde + RED + runtime por serviço, via Prometheus). Localização confirmada: `services/admin-bff/` e `services/admin-ui/`.

## Referências
- ADR-0002 (hexagonal), ADR-0006 (Prometheus/Grafana), ADR-0010 (control plane / estado em memória), ADR-0015 (database-per-service)
- ADR-0007 (SDD) / docs/sdd/SDD-GUIDE.md — escopo do SDD
- docs/OBSERVABILITY.md (métricas RED + custom por serviço)
