# Spec: 094-event-rules-canvas-legivel

**Serviço:** admin-ui
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** vídeo 2 (conquistas). No canvas de Event Rules o grafo fica pequeno demais para quem nunca viu a tela
entender cada nó, mesmo com zoom e "fit" manuais.

## Contexto (diagnóstico)

Medido no console a 1280×720, a resolução dos vídeos:

1. **O enquadramento não conta o painel de detalhe.** O fit reserva a lista de regras à esquerda (320 px + 90 de
   respiro), mas não o painel de detalhe à direita (290 px). O grafo é centralizado no meio que sobra e a última
   coluna (as conquistas) fica **escondida atrás do painel**.
2. **A lista de regras não recolhe.** Ela ocupa só o canto superior esquerdo, mas o fit reserva a largura dela na
   altura toda do canvas. Numa tela de 1280 px o canvas tem ~1015 px, e a lista come ~410 deles.
3. **O zoom automático nunca passa de 100%** (`maxZoom: 1`). Um grafo pequeno (o rascunho de uma regra nova, com
   dois nós) fica no tamanho mínimo de leitura (texto de 11 px) no meio de um canvas vazio.
4. Resultado: o grafo de uma regra aparece a ~60% (texto de ~7 px), e o rascunho, a 100% num canvas vazio.

## Comportamento

- **R1. Lista recolhível.** O painel da lista de regras SHALL ter um botão "Recolher lista" que o reduz a um
  botão compacto ("Regras · N"); clicar nele reabre a lista. Recolhida, a lista SHALL deixar de ser reservada
  pelo enquadramento. A escolha SHALL persistir no navegador (localStorage, por conveniência; sem ele, começa
  aberta).
- **R2. Enquadramento conta o painel de detalhe.** Com o painel de detalhe aberto e expandido, o fit SHALL
  reservar a largura dele à direita, como já faz com a lista à esquerda. Recolhido (só o cabeçalho) ou fechado,
  SHALL não reservar.
- **R3. Zoom automático até 150%.** O enquadramento automático e o botão de fit do canvas de Event Rules SHALL
  permitir até 150% de zoom, para que um grafo pequeno ocupe o espaço disponível. Os outros canvas (prévia de
  conquistas, pipeline) mantêm o limite atual.
- **R4. Tela cheia.** O canvas SHALL ter um botão "Tela cheia" junto dos controles de zoom, que expande o canvas
  para a janela inteira (por cima do menu lateral e das abas) e reenquadra o grafo; `Esc` ou o mesmo botão
  SHALL voltar ao tamanho normal. Tudo continua funcionando em tela cheia (lista, detalhe, rascunho, edição).
- Mudanças de layout (recolher ou abrir a lista, expandir ou recolher o detalhe, entrar ou sair da tela cheia)
  SHALL reenquadrar o grafo, como já acontece ao trocar de regra.

## Fora de escopo

- Tamanho da fonte dos nós e o layout do grafo (colunas, espaçamento): com R1 a R3 o zoom resolve a leitura.
- O canvas compacto da prévia de conquistas e o pipeline (só herdam o R4 se passarem a usar o botão; nesta fatia,
  não).

## Critérios de Aceite

1. A 1280×720, com uma regra global selecionada, lista recolhida e detalhe recolhido, o grafo inteiro (fatos,
   regra, evento e conquistas) aparece sem nada escondido e com zoom maior que 100%.
2. Com o detalhe expandido, nenhuma coluna do grafo fica atrás dele.
3. O rascunho de uma regra nova aparece ampliado (zoom acima de 100%) e centralizado no espaço livre.
4. Tela cheia cobre a janela, `Esc` volta, e o grafo é reenquadrado nas duas transições.
5. Recarregar a página mantém a lista recolhida ou aberta como estava.
6. `npm run lint`, `npm test` e `npm run build` passam; a prévia de conquistas e o pipeline não mudam.
