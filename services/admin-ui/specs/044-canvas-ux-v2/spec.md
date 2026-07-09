# Spec 044 — Canvas de Event Rules: criação visual + layout de tela cheia

**Status:** Draft — aguardando aprovação
**Serviço:** admin-ui (UI-only, mesmos endpoints)
**Relaciona:** spec 043 (fase 1 do canvas), ADR-0028

## Contexto (feedback do teste da fase 1, 2026-07-09)

Três pontos do teste do Vitor na fase 1:

1. **Criação ainda é "form"** — esperava criar regra também de forma visual,
   vendo-a nascer no canvas (referência: prototype do Workflow Editor na home
   do React Flow).
2. **Colunas laterais competem com o canvas** — paleta (esquerda) e painel de
   propriedades (direita) espremem o flow; a informação de contexto podia
   morar em cima (toolbar) e o detalhe podia flutuar sobre o canvas.
3. **Fullscreen** — ideia registrada para depois (fora de escopo aqui): modo
   canvas ocupar a tab inteira.

## Comportamento

### Layout (mata as duas colunas)

- **Toolbar** acima do canvas: seletor de arquétipo + botão "Nova regra" +
  (existente) toggle Canvas|Tabela e Atualizar. Sem coluna esquerda.
- **Painel de propriedades flutuante**: overlay no canto do canvas (React Flow
  `<Panel>`), card compacto fechável — o canvas ganha a largura toda; nada de
  coluna direita fixa.
- A paleta de facts deixa de ser lateral: os facts conhecidos viram chips
  **dentro do drawer de criação** (clique adiciona ao match) — só aparecem
  quando são úteis.

### Criação visual (drawer de baixo + nó rascunho ao vivo)

- **R1.** QUANDO o admin clicar em "Nova regra", o sistema DEVE abrir um
  drawer de baixo pra cima (Mantine `Drawer position="bottom"`) com o form
  (match + event types, com chips/sugestões dos facts e types conhecidos) SEM
  cobrir o canvas por inteiro.
- **R2.** ENQUANTO o drawer estiver aberto, o sistema DEVE renderizar no
  canvas um **nó rascunho** (estilo tracejado/"ghost") da regra sendo criada,
  atualizado em tempo real: cada par de match adicionado liga o rascunho ao nó
  de fact correspondente (criando nós de fact novos se o par for inédito);
  cada event type liga o rascunho ao nó de type (idem).
- **R3.** QUANDO o admin confirmar, o sistema DEVE persistir pelo endpoint
  existente e o rascunho vira nó definitivo no reload do grafo; QUANDO
  cancelar/fechar, o rascunho e os nós inéditos dele DEVEM sumir sem sobrar
  estado.
- **R4.** Arrastar um fact (chip do drawer ou nó do canvas) pro rascunho OU
  clicar num nó de fact/type com o drawer aberto DEVE adicioná-lo ao draft —
  o canvas vira superfície de montagem, não só visualização.

### Honestidade visual (inalterada)

O rascunho monta exatamente um `{match, eventTypes}` do motor atual — nenhuma
capacidade nova de motor aparece aqui (isso é V2a+, ADR-0028).

## Fora de escopo

- Fullscreen do canvas na tab (ideia registrada, avaliar depois).
- Camadas de leaderboard/analytics no grafo — as event rules são vocabulário
  exclusivo do achievements-service (ADR-0027); leaderboard/analytics consomem
  o envelope bruto do fan-out (ADR-0020/0021), não os event types. Uma futura
  "vista de pipeline" do fan-out é outro grafo (candidato natural ao 3º uso do
  kit flow), não uma camada deste.
- Qualquer mudança de backend.

## Critérios de aceite

1. Sem colunas laterais: canvas usa a largura toda; contexto na toolbar;
   detalhe do nó em overlay flutuante fechável.
2. Fluxo completo de criação: abrir drawer → montar regra vendo o rascunho e
   as ligações ao vivo → salvar → regra aparece definitiva; cancelar não deixa
   resíduo.
3. Paridade de validação com o form atual (≥1 par de match, ≥1 event type,
   erros do backend exibidos no drawer).
4. Toggle/Tabela e remoção de regra continuam funcionando como na fase 1.
5. `npm run build` verde; sem dependência nova.
