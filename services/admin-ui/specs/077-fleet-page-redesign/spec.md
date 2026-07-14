# Spec: 077-fleet-page-redesign

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Relaciona:** spec 059 (Fase 0 original — real + mock), ADR-0026 (Fleet por
jogo), ADR-0029 (infra Agones centralizada; fatia 2 = base deste redesign)
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 076.

## Contexto

Pedido do usuário: a aba Fleet estava "bem inicial" — uma tabela simples
(Jogo | Fleet) com um expander por linha misturando cadastro real, status
real (spec 023) e o esqueleto mockado da spec 059. Queria um retrabalho
geral: destaque melhor pra cada jogo (cada um cadastra sua própria imagem
de servidor), abas como o resto do console (uma "visão geral" de
imagens/versões online, outra de "configuração" editável direto), seguindo
os padrões visuais já estabelecidos — mas com liberdade explícita pra
experimentar mais aqui (é a página mais "vitrine" do roadmap) e já deixar
a estrutura preparada pra evolução futura.

Antes de desenhar, li a spec 059 (Fase 0 original) e o ADR-0029 (infra
Agones centralizada) pra entender o que já é real hoje vs. o que é
roadmap:

- **Real hoje**: `Game.fleet` (nome da Fleet Agones, ADR-0026, editável) e
  `FleetStatus` (spec 023 — cluster/localização, réplicas ready/desired/
  allocated, via `/api/games/{id}/fleet-status`, consulta de verdade o
  Agones). Testado ao vivo: ambos os fleets cadastrados hoje
  (Packet Arena, Capoeira Souls) estão `OFFLINE` — nenhum cluster Agones
  rodando nesta sessão, comportamento correto (não bug).
  Cluster location, quando existe, é `local` (mac) ou `pi5`.
- **Mock hoje (spec 059) e Fatia 2 do ADR-0029 (futuro, gated)**: imagem
  atual, health, tags disponíveis, botão "Promover". O ADR-0029 já
  desenha o schema futuro: `Game.imageRepository` (ex.:
  `ghcr.io/vitormartins1/<data-plane-repo>`), CI de cada repo de data
  plane publica no GHCR a cada release, a Fleet manifest passa a
  referenciar `<imageRepository>:<tag>` escolhido no console.

**Achado real na v1 (spec 059) que motivou parte do redesign**: o badge
real "OFFLINE" (vermelho) e o badge mock "2/2 saudáveis (mock)" (verde)
apareciam lado a lado sem nenhuma distinção visual — dado real e
ilustrativo se confundiam à primeira vista.

## Decisões de design

- **IA**: página ganha `Tabs` (mesmo padrão de `GameDetail`) — "Visão
  geral" (`IconLayoutGrid`) e "Configuração" (`IconSettings`, mesmo ícone
  já usado na aba Configurações de cada jogo).
- **Hero da página**: `ChannelStrip` (componente compartilhado da spec
  074) — `IconServer2` (mesmo ícone do nav), leitura real = contagem de
  fleets cadastradas. Primeiro uso do componente fora de Analytics/
  Leaderboards, validando que virou mesmo linguagem visual do console,
  não um one-off.
- **Visão geral**: grid de cards (`SimpleGrid`, 1/2 colunas) — **só jogos
  com fleet cadastrada** (jogo sem fleet não tem imagem/versão pra
  mostrar; aparece só como contador "+N jogos sem fleet — ver
  Configuração", sem card fantasma vazio diluindo a tela).
- **Separação visual real vs. mock (achado corrigido)**: cada card tem
  uma zona de telemetria real (localização + réplicas, sem borda extra,
  cor normal do card) e, abaixo, uma zona claramente mockada — borda
  tracejada, fundo `dark-8`, badge amarelo "mock" no canto. Real e
  ilustrativo nunca mais ficam adjacentes sem marcação.
- **Fidelidade do mock elevada** (pedido explícito — "já deixar
  preparado pro futuro"): a referência de imagem agora segue o formato
  real que a Fatia 2 do ADR-0029 vai usar
  (`ghcr.io/vitormartins1/<fleet>-server:<tag>`, em `Code`), e a lista de
  tags ganhou um tratamento tipo GitHub Releases (badge por tag, a atual
  destacada em `nominal`/preenchida, tooltip com "publicada há Nd").
  Continua 100% determinístico por nome de fleet, sem chamada de rede —
  mesma garantia da spec 059 (validado: nenhuma requisição dispara ao
  interagir com o Select/Promover).
- **Configuração**: tabela no mesmo padrão de "Modos de multiplayer"
  (`MultiplayerSettings.tsx`) — capa mini + nome do jogo, campo real
  editável (Fleet Agones, funcional, validado com save+reload real), e
  uma 3ª coluna **desabilitada** "Repositório de imagem" com tooltip
  explicando que é a Fatia 2 do ADR-0029 (gated) — greenfield visível,
  zero funcionalidade nova implementada.
- **Sem biblioteca nova**: avaliei introduzir algo (ícones de
  registry/CI mais elaborados, sparkline de health) e decidi que
  Mantine + `@tabler/icons-react` (já instalados) cobriam tudo com
  qualidade — o ganho "experimental" veio do layout (cards + zona
  tracejada + tags estilo release), não de uma dependência nova.

## Comportamento

1. Nav "Fleet" continua levando pra esta página (rota inalterada).
2. Aba "Visão geral": card por jogo com fleet, telemetria real (spec
   023) sempre visível; zona mockada sempre com borda tracejada + badge
   "mock". Jogo sem fleet não aparece em card.
3. Aba "Configuração": todos os jogos (com ou sem fleet), campo real
   editável (persistido de verdade, `PUT` existente da spec 059), campo
   de repositório de imagem sempre desabilitado.
4. Nenhuma interação na zona mockada (Select de versão, botão Promover)
   dispara chamada de rede ou persiste estado além do componente local
   — mesma garantia da spec 059, revalidada.

## Critérios de Aceite

1. Visão geral mostra os 2 jogos com fleet (dado real de hoje) com
   telemetria real distinguível da zona mock.
2. Configuração lista os 5 jogos; salvar um fleet novo persiste (
   validado: setei `sandbox-test-fleet` em Sandbox, dei reload completo
   da página, o valor persistiu; revertido ao final do teste).
3. Responsivo até 390px (cards empilham, régua do ChannelStrip encolhe).
4. Zero requisição de rede disparada pela zona mockada.
5. Sem erro de console.

## Fora de escopo

- Qualquer implementação real da Fatia 2 do ADR-0029 (imageRepository,
  GHCR, promoção de verdade) — continua tudo mock/desabilitado.
- Mudar o `FleetNavBadge` do nav lateral (App.tsx) — fora desta página.
