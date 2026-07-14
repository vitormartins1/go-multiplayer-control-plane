# Spec: 079-fleet-health-terminology-and-layout

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Relaciona:** specs 077/078 (redesign da Fleet, mesmo dia)
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 078.

## Contexto

Terceira rodada de feedback sobre a Fleet (specs 077/078), com 4 pedidos:

1. Terminologia da caixinha "Saúde (game servers)" — pedido explícito
   pra pesquisar os termos reais de Agones/Kubernetes antes de mockar
   ("Current"/"Desired" no formato `2/4`, como o `kubectl get deployment`
   mostra a coluna READY).
2. Layout lado a lado: "Saúde" ocupando ~40% à esquerda, "Imagem &
   versões" ~60% à direita, na mesma linha (em vez de empilhadas) —
   pedido pra testar e decidir a melhor opção.
3. Documentar (sem implementar) uma evolução futura de edição inline
   nos cards de "Clusters Agones" — hover revela lápis, clique edita.
4. Repensar a seção "Jogos" (ainda "não muito boa") — pensar em como um
   cadastro deixaria os jogos aparecerem na Visão geral; não implementar
   a parte de backend.

## Pesquisa (WebSearch, agones.dev — fonte oficial)

`Fleet.status` real tem `Replicas` (atual), `ReadyReplicas`,
`ReservedReplicas`, `AllocatedReplicas` — **"Desired" vem de
`Fleet.spec.replicas`, não do status**. Um `GameServer` individual passa
por `Scheduled → RequestReady → Ready`, dali pode ir a `Allocated`
(jogadores conectados) ou `Reserved` (não é removido em scale-down, mas
também não conta pro autoscaler), ou cair em `Unhealthy` se o container
morrer enquanto `Ready`/`Allocated`/`Reserved` (falha de health check —
estado transiente, o Fleet controller recicla o GameServer). `Unhealthy`
não é um campo agregado do `FleetStatus` real — é um estado individual do
GameServer, contado por quem lista os GameServers.
Fontes: [Fleet Specification](https://agones.dev/site/docs/reference/fleet/),
[GameServer Health Checking](https://agones.dev/site/docs/guides/health-checking/).

## Decisões

- **`mockFleetHealth` reescrito** com os campos corretos: `desired`
  (`Fleet.spec.replicas`, mock), `current` (soma de ready+allocated+
  reserved), `ready`, `allocated`, `reserved`, `unhealthy` (raro — só
  ~20% das seeds, reflete a natureza transiente do estado real).
- **Exibição em ratio**: "Réplicas" mostra `current/desired` (ex.:
  `3/3`, `2/2`) — mesma convenção da coluna READY do
  `kubectl get deployment`, e já consistente com o que a seção real
  ("Onde está rodando") já fazia pro dado real.
- **Achado real corrigido no caminho**: `(seed >> 2) % 2` usava
  right-shift **com sinal** — para um seed acima de 2^31, `ToInt32`
  reinterpreta o bit alto como sinal negativo, e o operador `%` do JS
  preserva o sinal do dividendo, gerando `Reserved: -1` (visto ao vivo
  na fleet `packet-arena`). Trocado por `>>> 2` (sem sinal), mesmo
  operador já usado no hash do seed.
- **Layout lado a lado**: `Grid` com `Grid.Col span={{base:12, md:5}}`
  (Saúde) e `span={{base:12, md:7}}` (Imagem & versões) — ~40/60,
  empilha em telas estreitas. Testado e mantido — ficou mais denso sem
  perder legibilidade, e a caixa de imagem precisa mesmo de mais
  largura (referência de imagem completa + 3 badges de tag).
- **Evolução futura documentada, não implementada** (comentários no
  código + aqui):
  - `ClusterCard`: hover revela ícone de lápis ao lado do badge "mock";
    clique troca os campos pra inputs editáveis inline (mesmo padrão
    `TextInput`+Salvar do `FleetConfigRow`). Só faz sentido quando
    existir um endpoint de perfis de cluster de verdade —
    `MOCK_AGONES_CLUSTERS` é hoje uma constante hardcoded, sem
    persistência.
  - `FleetConfigTable`/seção "Jogos": um fluxo de setup guiado — sugerir
    nome de fleet a partir do slug do jogo, escolher um cluster da seção
    acima (hoje só descritivo), salvar e destacar o card novo na Visão
    geral.
- **Passo intermediário implementado agora** (fatia pequena, real, sem
  esperar o fluxo guiado completo): jogo sem fleet ganha um badge
  "sem fleet" ao lado do nome + o botão muda de "Salvar" pra "Ativar"
  quando o campo está vazio — deixa a ação de "isto é o que faz o jogo
  aparecer na Visão geral" mais clara, sem construir nenhum fluxo novo.

## Critérios de Aceite

1. Caixinha "Saúde" mostra `current/desired` + Ready/Allocated/
   Reserved/Unhealthy com terminologia conferida na documentação oficial
   do Agones.
2. Nenhum valor negativo (bug do right-shift corrigido e revalidado ao
   vivo nos 2 fleets reais).
3. "Saúde" e "Imagem & versões" lado a lado (~40/60) em telas ≥ md,
   empilhados em mobile (390px testado).
4. Jogo sem fleet mostra badge "sem fleet" + botão "Ativar" em vez de
   "Salvar".
5. Sem erro de console.

## Fora de escopo

- Implementar a edição inline dos clusters — só documentado.
- Implementar o fluxo guiado de setup de fleet — só documentado.
- Endpoint real de perfis de cluster — não existe, não faz parte desta
  fatia.
