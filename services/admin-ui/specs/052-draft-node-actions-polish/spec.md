# Spec 052 — Ações do form in-canvas no header + event type novo sempre visível

> Status: **implementada** (pedida e aprovada por descrição direta do Vitor, 2026-07-10)
> Fatia: pequena/aditiva — só admin-ui, reposicionamento visual + simplificação
> de estado do form in-canvas (specs 048/050); sem mudança de contrato com o
> backend.

## Contexto

Feedback do Vitor testando o form in-canvas (criação/edição de regra, specs
048/050): os botões Cancelar/Salvar no rodapé do nó e a tag "rascunho"/
"editando" no header são redundantes com o próprio outline laranja tracejado
que já sinaliza "form ativo" (spec 044, classe `is-draft`). Separadamente, o
botão "+" que cria o primeiro event type novo (violeta, flutuante no handle
direito) ficou visualmente destacado demais; como o autocomplete do event
type novo já sugere os event types existentes (dropdown), o próprio campo
cobre os dois casos (linkar um existente digitando/selecionando, ou criar um
novo digitando algo inédito) — o "+" só existe pra revelar esse campo, o que
pode ser eliminado deixando-o sempre visível.

## Comportamento (EARS)

### Ações no header

1. O header do form in-canvas (criação E edição) DEVE mostrar, alinhados à
   direita: o botão **Cancelar** e o botão **Salvar** (sem o texto "regra" —
   só os dois nomes curtos), no lugar onde hoje fica a tag "rascunho"/
   "editando".
2. A tag "rascunho"/"editando" DEVE ser removida — a identidade do form
   (outline tracejado laranja, spec 044) e o título ("Nova regra"/"Editar
   regra") já comunicam o estado, sem precisar de badge extra.
3. O footer do nó (que hoje só contém os dois botões) DEVE ser removido.

### Event type novo sempre presente

4. Ao abrir o form (criação OU edição), DEVE nascer automaticamente UMA
   caixinha de "event type novo" (mesmo componente `EventTypeDraftNode` já
   existente, com autocomplete e sugestões dos types existentes) — sem
   precisar clicar em nada pra revelá-la.
5. Essa caixinha NÃO DEVE ter um botão de fechar/remover — é sempre exatamente
   uma por form, do início ao fim da edição.
6. O botão "+" (flutuante, violeta) que hoje cria a caixinha DEVE ser
   removido, junto com o handler que ele chamava.
7. Deixar a caixinha vazia ao salvar continua válido (o save já filtra
   valores vazios do array de types novos antes de montar `eventTypes`) —
   o usuário pode confiar só nos types já linkados (clicando em event types
   existentes no grafo), sem preencher a caixinha nova.

## Critérios de aceite

- [x] Abrir "Nova regra": header mostra "Nova regra" + Cancelar/Salvar à
      direita (sem tag); uma caixinha de event type novo já aparece, sem X.
- [x] Editar uma regra do jogo: mesmo layout, header "Editar regra"; a
      caixinha de event type novo nasce vazia (os types já linkados
      continuam como conexões normais no grafo, não como ghosts).
- [x] Cancelar / Salvar funcionam idênticos a antes (mesmos handlers).
- [x] Alturas do nó recalibradas (`rulesGraph.ts`) com `offsetHeight` real —
      gotcha conhecido (spec 050): remover o footer muda a altura renderizada.
- [x] Build verde + smoke Playwright confirmando o layout novo.

## Fora de escopo

- Qualquer mudança no formato salvo (`match`/`eventTypes`) ou nos endpoints.
- Permitir mais de uma caixinha de event type novo por form (se precisar no
  futuro, é uma fatia própria — não há pedido pra isso agora).
