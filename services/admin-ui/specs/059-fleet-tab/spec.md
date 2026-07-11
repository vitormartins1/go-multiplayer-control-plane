# Spec 059 — aba Fleet (fatia real do campo `fleet` + esqueleto mockado da evolução futura)

**Serviços:** admin-ui (nova aba/página) + admin-bff (proxy do campo `fleet`, hoje ausente)
**Status:** Done
**Relaciona:** ADR-0026 (fleet por jogo, Accepted/implementado), ADR-0029 (infra Agones centralizada, Proposed — fatia 2 ainda não implementada)
**Depende de:** nada novo no game-registry — `Fleet`/`setFleet` já existem lá (spec 012) e nunca foram expostos além dele.

## Contexto

Hoje, saber se um jogo tem Fleet do Agones cadastrada — e qual — não tem nenhum caminho pelo console: o game-registry já tem o campo (`Game.Fleet`, `PUT .../fleet`, ADR-0026) e já é usado de verdade pelo `session-service` no `Allocate`, mas **nem o admin-bff nem o admin-ui jamais expuseram esse campo** (conferido: zero ocorrências de "fleet" em `services/admin-bff` e em `services/admin-ui/src`). Foi cadastrado via chamada direta à API no rollout (Packet Arena, Capoeira Souls) — não existe tela pra ver ou editar isso hoje.

Motivação do Vitor (conversa que originou esta spec): ele quer uma aba própria "Fleet", separada de "Infrastructure" (que é sobre saúde da *plataforma* — serviços Go, fan-out; Fleet é sobre os *game servers*, um conceito de produto mais próximo do que o console do GameLift mostra — bate com a tese do projeto, ADR-0008). Além do cadastro, ele quer visualizar hoje **como seria** a evolução futura dessa aba: status de build/imagem por jogo, lista de versões disponíveis (publicadas via CI do repo do dataplane num registry, provavelmente GHCR) e um botão manual de "promover" a Fleet pra uma versão escolhida — sempre manual, nunca CD automático (mesmo modelo do GameLift: build e Fleet são coisas separadas, a escolha de qual build uma Fleet roda é explícita).

Essa fatia (059) entrega duas coisas de natureza bem diferente, e a distinção é o ponto central da spec:

1. **Real**: o campo `fleet` passa a existir de ponta a ponta (game-registry → admin-bff → admin-ui), com leitura e edição de verdade. Fecha um gap real (ADR-0026 nunca teve frontend).
2. **Mock, explicitamente marcado como tal**: imagem/tag atual, health de réplicas, lista de versões disponíveis, botão de promoção — nada disso existe hoje em lugar nenhum (nem proposto em detalhe na ADR-0029), é só o esqueleto visual da direção futura, sem nenhuma chamada de rede ou efeito real por trás.

## Visão de evolução futura (contexto — nenhuma destas fases está em escopo aqui)

- **Fase A** — CI dos repos de dataplane builda a imagem Docker do dedicated server e publica no GHCR (`ghcr.io/<org>/<repo>:<tag>`) a cada release. `Game` ganha `imageRepository` (ADR-0029 fatia 2, hoje só esboçada, gated por evidência).
- **Fase B** — admin-bff/admin-ui consultam a API de packages do GHCR pra listar as tags publicadas de um `imageRepository`.
- **Fase C** — um endpoint novo (onde vive, a decidir — game-registry ou um serviço fino separado) lê o estado real do Agones (via client-go ou kubectl) — imagem hoje rodando na Fleet, réplicas, health dos GameServers.
- **Fase D** — o botão "Promover" (hoje mockado) aciona de verdade a troca do manifest da Fleet pra apontar pra tag escolhida, reaplicando via Agones — sempre um clique explícito, nunca automático.

Cada fase acima é uma decisão própria (toca infra/K8s, custo, autenticação com o registry) e não é aprovada por esta spec — só documentada aqui pra dar contexto ao esqueleto da Fase 0.

## Escopo desta fase (Fase 0)

### Fatia real
- `admin-bff`: `Game`/DTO relevante ganha `fleet string` (proxy direto do game-registry); novo endpoint (ou reaproveitado) pra `PUT` do fleet do jogo.
- `admin-ui`: `Game` (api.ts) ganha `fleet?: string`; nova página consome a lista de jogos + fleet de cada um; formulário simples pra editar o fleet do jogo (reaproveita o `PUT` acima) — equivalente em espírito ao que já existe pra outros campos simples do jogo (ex. rename).

### Fatia mockada (visual, sem dado/chamada real)
- Por jogo com fleet configurada: card/linha mostrando (todos mock, visivelmente marcados):
  - "Imagem atual": ex. `v0.4.2 (mock)`.
  - Health: badge tipo "3/3 saudáveis (mock)".
  - Lista suspensa de tags "disponíveis": 2-3 valores mockados (ex. `v0.4.2` atual, `v0.4.3`, `v0.5.0-rc1`).
  - Botão "Promover" — sempre desabilitado, com tooltip "Integração com o registry de imagens ainda não implementada".
- Alerta fixo no topo da página (ex. `Alert` amarelo): "Imagem/versão/health desta seção são ilustrativos — a integração real (build no registry + leitura de estado do Agones) ainda não existe. Ver spec 059."
- Jogos sem fleet cadastrada aparecem só com um estado "sem fleet configurada" — nem tenta mostrar a parte mockada (não faz sentido mockar build de uma fleet que não existe).

## Comportamento (EARS)

1. **O SISTEMA DEVE** adicionar um item "Fleet" ao nav principal (`App.tsx`), no mesmo nível de Games/Players/Rooms/Analytics/Infrastructure.
2. **QUANDO** o usuário abre a aba Fleet, **O SISTEMA DEVE** listar todos os jogos cadastrados (`GET /games`) com o nome da fleet real (campo novo `fleet`).
3. **QUANDO** um jogo não tem fleet cadastrada, **O SISTEMA DEVE** mostrar um estado "sem fleet configurada" com um jeito de cadastrar uma (reaproveita o `PUT` real) — sem nenhuma seção mockada abaixo.
4. **QUANDO** um jogo tem fleet cadastrada, **O SISTEMA DEVE** mostrar, além do nome real da fleet (editável), a seção mockada de imagem/health/versões disponíveis, com o disclaimer visível de que é ilustrativo.
5. **O SISTEMA DEVE** exibir um alerta fixo no topo da página avisando que os dados de build/imagem são mockados (não só no card — visível mesmo antes de expandir qualquer coisa).
6. **QUANDO** o usuário clica em "Promover" em qualquer linha mockada, **O SISTEMA NÃO DEVE** fazer nenhuma chamada de rede, alterar nenhum estado real, nem mostrar toast de sucesso — o botão fica desabilitado (preferível) ou, se clicável, só mostra feedback de "não implementado".
7. **O SISTEMA NÃO DEVE** persistir nenhuma escolha feita na parte mockada (tag selecionada etc.) além do estado local do componente — reload da página volta ao mock inicial.
8. Editar o campo `fleet` real (item 3/4) **DEVE** seguir a mesma validação já existente no backend (`setFleet`, spec 012 do game-registry) — o form não reimplementa validação própria.

## Critérios de aceite

- Nav principal ganha "Fleet"; rota nova, carregada sob demanda (mesmo padrão `lazy()` das outras páginas).
- Jogos com `fleet` cadastrada hoje (Packet Arena, Capoeira Souls) aparecem com o nome real + a seção mockada completa.
- Jogos sem fleet aparecem só com o estado "sem fleet configurada" + forma de cadastrar (chamada real).
- Editar o fleet de um jogo pelo novo formulário reflete de verdade (persistido no game-registry, visível num reload).
- Nenhuma chamada de rede nova disparada pela parte mockada (network tab limpo ao interagir com a lista de versões/botão Promover).
- Clicar em "Promover" não muda nada visível além, no máximo, de um toast/tooltip de "não implementado".
- `tsc --noEmit` + `npm run build` limpos; smoke manual via Playwright cobrindo jogo com fleet, jogo sem fleet, e o não-efeito do botão Promover.

## Fora de escopo

- Qualquer leitura real de estado do Agones (imagem rodando, réplicas, health) — Fase C acima.
- Integração com GHCR/GitHub Packages API pra listar tags reais — Fase B acima.
- `Game.imageRepository` e publicação de imagem via CI — Fase A acima (ADR-0029 fatia 2).
- Endpoint que de fato reaplica o manifest da Fleet ("Promover" de verdade) — Fase D acima.
- Adendo às ADR-0026/0029 descrevendo a UX de promoção manual — só depois que uma fase real for aprovada e implementada (não faz sentido documentar decisão de arquitetura pra um mock).
- Mover a aba pra dentro de "Infrastructure" em vez de item próprio no nav — avaliado e descartado (Fleet é conceito de produto/jogo, Infrastructure é de plataforma; ver Contexto).
