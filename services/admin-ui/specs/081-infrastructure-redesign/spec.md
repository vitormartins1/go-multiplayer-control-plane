# Spec: 081-infrastructure-redesign

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Relaciona:** specs 077/078/079 (mesmo padrão real+mock aplicado à Fleet)
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 080.

## Contexto

Pedido do usuário: ampliar a página Infrastructure (nav principal, rota
`/infrastructure`, componente `Overview.tsx`) no mesmo espírito do
redesign da Fleet — ler a documentação do projeto, mockar dado grounded
no que já está decidido, e imaginar como a página seria num produto
finalizado. Dois problemas apontados de saída: o canvas de topologia
(`TopologyGraph`, React Flow) "perdeu o sentido de estar ali" e a tabela
"Saúde dos serviços" não precisava ser uma tabela — as duas mostravam a
MESMA informação (serviço + up/down) de dois jeitos redundantes.

Também citado: o badge "local" no header (`App.tsx`) é hoje fixo,
futuramente vai refletir o ambiente real (AWS) — contexto, não pedido de
mudança no próprio badge.

## Pesquisa (docs do projeto, antes de mockar)

- **`docs/ARCHITECTURE.md` §3** — mapa canônico de serviços: cada
  serviço Go forma um "par de comparação" com um equivalente AWS
  (GameKit Identity/Achievements/User Gameplay Data/Game State Saving,
  GameLift FlexMatch/orquestração, ref. arch. DynamoDB/Redis, Game
  Analytics Pipeline). **Dado real e documentado**, só que sem nenhum
  endpoint que o exponha — por isso ainda leva o tratamento visual de
  `MockZone` (mock = "sem API viva hoje", não "informação inventada").
- **ADR-0005** — menu de compute por serviço (Lambda/ECS Fargate/
  ECS-on-EC2/EC2/EKS), escolha "diferida" — nenhum serviço tem a decisão
  final commitada em produção.
- **ADR-0008** — a tese do projeto: comparar custo/performance do Go
  contra o equivalente AWS gerenciado.
- **`docs/ROADMAP.md`** — Fase 11 ("Comparação Go vs. AWS") está
  **pendente**, só acontece depois do primeiro deploy prod (Fase 3:
  "prod AWS = backlog"). FinOps cita estimativa real de **$20–37/mês**
  pro Go, budget alerta em 80% de $40/mês.
- **`docs/OBSERVABILITY.md`** — 9 dashboards Grafana já provisionados
  (Service Overview RED, PostgreSQL, Tracing/APM, etc.) — confirma que
  métricas profundas já têm um lugar (Grafana), o admin-ui não deveria
  duplicá-las (mesmo princípio que já estava no comentário original do
  `Overview.tsx`).
- **AWS GameLift pricing (WebSearch)** — cobra por hora de instância
  (~365h/mês médio com autoscaling) mesmo ocioso, mais FlexMatch por
  ticket — grounding real pra explicar por que a estimativa "AWS
  gerenciado" tende a ser bem mais alta que o Go self-hosted, sem
  inventar uma cotação exata.

## Decisões

- **`MockZone` extraído** (`src/components/MockZone.tsx`) — a borda
  tracejada + badge "mock" já se repetia 3x na Fleet; esta é a 4ª+
  aplicação (achado: hora certa de consolidar). `Fleet.tsx` refatorado
  pra usar o componente compartilhado (2 dos 3 usos — o card de cluster
  tem cabeçalho com ícone próprio, não force-encaixado).
- **`TopologyGraph.tsx` removido** (arquivo deletado, único consumidor
  era `Overview.tsx`) — canvas + tabela de saúde viram um grid de
  `ServiceCard` (`SimpleGrid`), um por serviço real (`topology.nodes`):
  badge de saúde real, dependências reais (`topology.edges`, ex.
  "depende de: identity-service") e, numa `MockZone`, o equivalente AWS
  + compute alvo.
- **Tabs** (mesmo padrão de Fleet): "Visão geral" (`IconLayoutGrid`) e
  "Comparação AWS" (`IconScale`).
  - Visão geral: `ChannelStrip` "Serviços" (readout real N/M up) + grid
    de cards; `ChannelStrip` "Matchmaking" e "Fan-out / entrega" com
    leitura real, StatCards inalterados (dado já era bom).
  - Comparação AWS: `ChannelStrip` "Comparação Go × AWS" (readout "Fase
    11 · pendente (ROADMAP.md)", fato real citado) + ambiente atual
    (real: Local/Docker Compose/$0) vs ambiente de produção (`MockZone`:
    AWS/Terraform) + estimativa de custo (Go: **$20–37/mês citado do
    ROADMAP.md, sem badge mock** — é estimativa real documentada, não
    inventada; AWS: `MockZone`, ordem de grandeza com raciocínio
    explícito) + tabela do mapa canônico de serviços + lista de
    evoluções futuras documentadas (badge de ambiente dinâmico, custo
    real via Cost Explorer, deep-link pro Grafana por `$job`).
- **Distinção real vs. citado vs. mock, deliberada**: 3 categorias
  visuais nesta página — (1) dado real ao vivo (Prometheus, sem
  marcação), (2) fato documentado citado com fonte (ex. "$20–37/mês,
  ROADMAP.md" — sem badge "mock", mas também não é telemetria ao vivo),
  (3) `MockZone` (ilustrativo, sem API por trás). Optei por não badgear
  (2) como mock porque seria enganoso chamar de "inventado" algo que o
  próprio projeto já decidiu e documentou.

## Critérios de Aceite

1. Nenhum canvas de topologia na página; nenhuma tabela duplicando a
   mesma informação de saúde já presente nos cards.
2. Cada `ServiceCard` mostra saúde real, dependências reais (quando
   existem) e equivalente AWS + compute numa `MockZone`.
3. Aba "Comparação AWS" cita a Fase 11 do ROADMAP.md como pendente, sem
   fingir que a comparação real já existe.
4. `TopologyGraph.tsx` removido do repo (não só desconectado).
5. Responsivo até 390px; sem erro de console; `tsc`/`build` limpos.

## Fora de escopo

- Badge de ambiente dinâmico no header (`App.tsx`) — documentado como
  evolução futura, não implementado (não existe fonte de "ambiente
  atual" genérica pra plataforma inteira hoje, só por-Fleet via spec
  023).
- Custo real via AWS Cost Explorer, deep-link pro Grafana — documentados
  como evoluções futuras.
