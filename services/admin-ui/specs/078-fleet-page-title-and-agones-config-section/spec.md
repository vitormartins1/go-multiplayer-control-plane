# Spec: 078-fleet-page-title-and-agones-config-section

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Relaciona:** spec 077 (redesign anterior da Fleet, mesmo dia)
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 077.

## Contexto

Feedback do usuário sobre o redesign da spec 077, em 3 pontos:

1. **Uso indevido do `ChannelStrip` como título de página.** `ChannelStrip`
   é o divisor de SEÇÃO dentro de uma página (nasceu na Analytics, spec
   071; reusado nos grupos de board do Leaderboard, spec 074) — usá-lo
   como título da própria página Fleet confundia os dois papéis. Ao
   comparar, achou também que **Rooms** já estava com o título fora do
   padrão: `Text fw={600} size="lg"` (corpo, tamanho "lg") em vez de
   `Title order={3}` (fonte de display do tema) — Games/Players/Analytics
   já usavam `Title order={3}` corretamente.
2. **Faltava uma caixinha de "Saúde" por jogo na Visão geral** — o
   detalhe por GameServer (Ready/Allocated/Reserved/Unhealthy — os
   estados reais da state machine do Agones), separado da "Imagem &
   versões". Pedido explicitamente como mock por enquanto.
3. **Aba Configuração "muito simplória"** — só a tabela de jogos, sem
   nenhuma seção ou contexto. Pedido: uma seção "Configuração do Agones"
   acima, mockando os perfis dos 2 clusters reais do projeto (local no
   Mac, Raspberry Pi 5 home lab — ADR-0029), separada da seção "Jogos"
   (o que já existia) usando o mesmo divisor de `ChannelStrip`.

## Decisões

- **Título da página**: `Title order={3}` + contagem inline (`· N fleets
  cadastradas`), mesmo padrão exato de `Players.tsx`. `ChannelStrip`
  removido do topo da página.
- **Rooms.tsx**: mesmo fix — `Text size="lg"` → `Title order={3}`, com a
  frase descritiva (mais longa que uma contagem simples, não cabe inline
  no Title) como `Text` dimmed logo abaixo, no padrão de `Analytics.tsx`
  (Title + `Text c="dimmed" mt={-8}`, aqui `mt={-4}` pra compensar a
  fonte um pouco menor da segunda linha).
- **Saúde (game servers), mock**: novo box na mesma zona tracejada dos
  outros mocks do card, com 4 contadores (Ready/Allocated/Reserved/
  Unhealthy) — determinístico por nome de fleet
  (`mockFleetHealth`), mesma disciplina do resto do mock desta página:
  nenhum endpoint expõe hoje o detalhe por GameServer (`FleetStatus`,
  spec 023, só tem o agregado da Fleet inteira).
- **Configuração do Agones, mock**: nova seção acima de "Jogos",
  precedida por `ChannelStrip` (ícone `IconCloud`, leitura "2 clusters
  (mock)"). 2 cards ilustrativos — "Local (Mac)" (`IconBrandApple`) e
  "Pi 5 (Home Lab)" (`IconCpu`) — com runtime/nós/versão do Kubernetes/
  versão do Agones, todos mock. **Deliberadamente sem nenhum badge de
  status "ao vivo"** (ex.: "reachable/unreachable") — só cadastro
  descritivo, pra não repetir o problema real-vs-mock que a própria
  spec 077 corrigiu na Visão geral (um badge de status ao lado de um
  perfil mockado pareceria dado real de novo).
- **Seção "Jogos"**: a tabela existente ganha o mesmo divisor
  `ChannelStrip` (ícone `IconServer2`, o mesmo do nav), com leitura real
  (`N/M com fleet`) — não mais um bloco solto sem cabeçalho.

## Critérios de Aceite

1. Fleet e Rooms têm `Title order={3}` como título de página, igual a
   Games/Players/Analytics.
2. Cada card da Visão geral mostra "Saúde (game servers)" com 4
   contadores mock, separado de "Imagem & versões".
3. Aba Configuração tem 2 seções com `ChannelStrip`: "Clusters Agones"
   (2 cards mock, sem badge de status ao vivo) e "Jogos" (tabela real,
   inalterada em funcionalidade).
4. Responsivo até 390px.
5. Sem erro de console novo (os 404 de `multiplayer-profile` na Rooms
   são pré-existentes, não introduzidos por este fix — confirmado via
   `git diff`, não tocados).

## Fora de escopo

- Qualquer leitura real de cluster (nós, versões) — permanece mock, sem
  endpoint disponível.
- Corrigir os 404 pré-existentes de `multiplayer-profile` na Rooms —
  não relacionado ao pedido, não investigado a fundo.
