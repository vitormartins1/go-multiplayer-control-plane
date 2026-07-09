# Spec 050 — Polimento do nó de edição + edição de regra in-canvas

> Status: **draft** (aguardando aprovação)
> Fatia: UI substancial (edição depende da spec achievements/016 — PUT).
> Origem: feedback do Vitor testando a 049 (2026-07-09).

## Problemas relatados

1. **Form do rascunho destoa do canvas** — inputs e botões Mantine "crus"
   dentro do nó quebram a imersão (não parecem parte do card).
2. **Ghost de event type novo nasce no fim da coluna** — esperado: ao lado
   do nó da nova regra. (Causa real: o baricentro não conhece os ghosts —
   caem no `MAX_SAFE_INTEGER` e afundam.)
3. **Card de stat com warning de drift fica mais largo** que os demais e o
   espaçamento vertical ao redor dele fica desigual (altura ESTIMADA no
   layout ≠ altura real renderizada — gotcha conhecido do rulesGraph).
4. **Não existe edição de regra** — só criar/remover. Desejo: mesma
   identidade visual da criação, no próprio nó.

## Comportamento (EARS)

### Identidade visual do form in-canvas (criação E edição)

1. Os inputs dentro do nó DEVEM seguir a identidade do card: fundo escuro
   (`#131a24`), borda `#2a3340`, fonte mono 11px, sem o visual "form Mantine
   padrão"; os botões do rodapé DEVEM ser compactos e discretos (subtle),
   coerentes com o footer do card. Autocomplete e op-chip continuam.

### Posição do ghost de event type novo

2. **Quando** o usuário clicar no "+" do rascunho, o ghost do event type
   novo DEVE nascer no TOPO da coluna de types, adjacente ao nó do rascunho
   (que já fica no topo da coluna de regras) — nunca no fim da coluna.

### Cards de fonte uniformes

3. Todos os cards de fonte DEVEM ter a MESMA largura fixa, com ou sem warning
   de drift; o texto do warning DEVE ser menor (não dita a largura do card).
4. O espaçamento vertical entre cards de uma coluna DEVE ser visualmente
   uniforme mesmo com alturas diferentes — calibrar as alturas estimadas do
   layout com as alturas reais renderizadas (medir via Playwright no smoke).

### Edição de regra in-canvas (requer achievements/016)

5. **Quando** o usuário passar o mouse sobre um nó de regra do JOGO, o badge
   "editável" DEVE virar a ação **editar**; regras globais permanecem
   read-only (badge inalterado).
6. **Quando** clicar em editar, o nó DEVE se transformar no MESMO form da
   criação (linhas do match pré-carregadas com chave/op/valor, event types
   ligados pré-marcados), com rodapé Cancelar / Salvar. Só um form ativo por
   vez no canvas (editar cancela rascunho aberto e vice-versa).
7. **Quando** salvar, a UI DEVE chamar `PUT /api/games/{id}/event-rules/{ruleId}`
   e recarregar; **quando** cancelar (ou Esc), o nó DEVE voltar ao estado
   normal sem alteração. O canvas DEVE se re-layoutar dinamicamente quando o
   nó cresce (modo edição) e encolhe (salvo/cancelado).
8. `archetype` não é editável; a edição pode ligar/desligar types existentes
   e criar types novos pelo "+" (mesmo fluxo do rascunho).

## Critérios de aceite

- [ ] Form de criação com a identidade do card (screenshot antes/depois).
- [ ] "+" cria ghost adjacente ao rascunho (topo da coluna de types).
- [ ] Cards de fonte com largura idêntica; gaps uniformes na coluna de
      métricas com o drift do `headshots` presente (medido no smoke).
- [ ] Hover em regra do jogo mostra "editar"; global não.
- [ ] Editar a regra do Mestre Curativo (ex.: revives ≥ 5 → ≥ 6), salvar,
      nó volta ao normal com a condição nova; cancelar não altera nada.
- [ ] Esc no modo edição = cancelar; rascunho e edição mutuamente exclusivos.
- [ ] Build verde + smoke Playwright do fluxo completo.

## Fora de escopo

- Editar regras globais (V2b — supressão por jogo — é o caminho certo).
- Editar achievements pelo canvas (ficam na aba Achievements).
