# Plan 044 — Canvas UX v2 (criação visual + layout)

## Decisões técnicas

1. **Nó rascunho via o próprio pipeline do grafo:** `buildRulesGraph` ganha um
   parâmetro opcional `draft` ({match, eventTypes}) que entra no layout como
   uma pseudo-regra `__draft__` — facts/types inéditos viram nós "ghost".
   Assim o rascunho participa do layout normal (baricentro) e as ligações
   nascem pelo mesmo código das regras reais; nada de segundo mecanismo de
   desenho. Nós/edges do rascunho são marcados (`className "is-draft"`, edges
   tracejadas animadas) e o RuleNode mostra badge "rascunho".
2. **Drawer de baixo sem bloquear o canvas:** Mantine `Drawer`
   position="bottom" com `withOverlay={false}`, `trapFocus={false}` e
   `lockScroll={false}` — o canvas continua interativo (R4: clicar em fact/
   type com o drawer aberto adiciona ao draft; drop de chip idem).
3. **Painel flutuante:** `FlowCanvas` ganha prop `overlay` renderizada num
   `<Panel position="top-right">` do React Flow — card fechável com o
   NodeDetails existente. Colunas laterais removidas; arquétipo + Nova regra
   viram toolbar acima do canvas.
4. **Chips de facts migram pro drawer** (clique adiciona; drag continua
   funcionando pro canvas). Sugestões continuam derivadas do dado real.

## Arquivos

| Arquivo | Ação |
|---|---|
| `src/lib/rulesGraph.ts` | editar — param `draft`, marcação ghost |
| `src/components/flow/FlowCanvas.tsx` | editar — prop `overlay` (Panel) |
| `src/components/flow/nodes.tsx` | editar — RuleNode com estado rascunho |
| `src/components/flow/flow.css` | editar — estilos `is-draft` |
| `src/pages/GameEventRulesCanvas.tsx` | reescrever — toolbar, drawer, overlay |

## Riscos / atenção

- Re-layout a cada mudança do draft faz nós "pularem" (sem animação) — aceito
  na fase; animar posição fica pra polish futuro se incomodar.
- `onNodeClick` muda de papel com o drawer aberto (adicionar ao draft) vs
  fechado (selecionar/destacar) — manter a distinção óbvia no código.
- Drawer sem overlay: garantir que não rouba foco do canvas nem trava scroll.
