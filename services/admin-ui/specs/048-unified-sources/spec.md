# Spec 048 — Fontes unificadas + criação IN-CANVAS (edição dentro do nó)

**Status:** Draft v2 — aguardando aprovação (v1 aprovada; interação de criação
revisada após novo feedback: edição dentro do nó em vez de drawer)
**Serviço:** admin-ui (UI-only; zero mudança de backend)
**Relaciona:** specs 043–047, achievements/014–015, ADR-0028,
`docs/EVENT-RULES-GUIDE.md` (conceitos que esta tela materializa)

## Contexto (feedback dos testes da 015)

1. A MESMA métrica aparece em dois nós sem ligação (Stat laranja da 045 +
   condição azul `stat.kills ≥ 10` da 014/015), e criar regra com stat obriga
   a digitar `stat.<chave>` na mão em vez de usar a caixinha da tela.
2. Um nó por CONDIÇÃO distinta não escala (limiares diferentes = nós quase
   idênticos).
3. O drawer (de baixo na 044, à direita na 047) continua deslocando a criação
   pra fora do canvas — a direção desejada é **editar dentro do próprio nó
   rascunho** (referência: exemplo call-center do Foblex, inputs dentro do
   node; padrão suportado nativamente pelo React Flow via classe `nodrag`).

**Modelo conceitual que a tela materializa** (motor inalterado; ver
`docs/EVENT-RULES-GUIDE.md`): um vocabulário de **fontes** (facts estruturais,
`mode`, stats), **condições como relações** — fonte→regra (match) e
stat→achievement (critério por métrica). Achievement é por evento OU por
métrica; a "mistura" mora nas regras. Sem tipo "Ambos" (rejeitado).

## Comportamento

### Modelo visual (nós por FONTE, condições nas edges)

- **Camada 1 — Fontes**: um nó por CHAVE: estruturais/`mode` (azul), stats
  (laranja, ícone de métrica — o nó Stat sai da camada de gatilhos e vem pra
  cá; a camada 3 volta a ser só event types).
- **Condições nas edges** fonte→regra: `outcome →(= win)→ regra`,
  `deaths →(≤ 0)→ regra`.
- **Critério por métrica como edge** stat→achievement com label
  (`acumulada ≥ 500`) — long edge proposital: métrica alimenta achievement
  direto, sem regra.
- Painel do nó de fonte lista as condições em que aparece; seleção segue
  destacando o caminho.

### Criação IN-CANVAS (substitui o drawer)

- **"Nova regra"** materializa imediatamente o nó rascunho no canvas em
  **modo edição** (card maior, ~340px): linhas de condição com
  [chave autocomplete][op][valor] editáveis DENTRO do nó (`nodrag` nos
  inputs; drag só pelo header), "+" compacto adiciona linha.
- **Operador dinâmico**: chip clicável no lugar de um Select — clique abre um
  popover compacto com `= ≠ < ≤ > ≥ in` (economiza largura dentro do nó).
- **Event types**: um "+" junto ao handle direito do rascunho cria um nó de
  event type EDITÁVEL inline (autocomplete com os types existentes); repetível
  pra N types. Clicar num nó de event type existente com o rascunho ativo
  também liga.
- **Fontes clicáveis**: clicar num nó de fonte com o rascunho ativo adiciona
  uma linha de condição pré-preenchida (estrutural/`mode` com `=`; stat com
  `≥`) e foca o valor.
- As edges do rascunho (fonte→rascunho com label da condição;
  rascunho→types) aparecem ao vivo conforme o preenchimento (como hoje).
- **Salvar** (rodapé do nó) persiste pelo endpoint existente e o rascunho
  vira definitivo; **Cancelar** descarta nó, ghosts e edges sem resíduo.
  Esc = cancelar.

### Avisos, atalho e navegação

- **Aviso de redundância (não-bloqueante)**: no painel/nó da regra, quando um
  event type emitido já é emitido por uma regra de match VAZIO do mesmo
  escopo aplicável ("sempre bate"), mostrar hint: "já emitido
  incondicionalmente por outra regra — esta emissão não adiciona nada (o
  motor deduplica)". Sem validação dura: emissão duplicada é inócua por
  design (união sem duplicar) e casos condicionais legítimos existem.
- **Atalho de achievement**: painel de event type órfão (0 listeners) ganha
  "Criar achievement que escuta este type" (form da aba Achievements,
  pré-preenchido). Catálogo continua dono na aba Achievements.
- **Busca na toolbar**: filtra por highlight (o que casa acende, resto
  esmaece; limpar restaura).

## Fora de escopo

- Tipo "Ambos" no form de achievement (rejeitado — ver guia).
- Validação dura de emissão duplicada (semântica do motor já é segura).
- Backend/motor.

## Critérios de aceite

1. Uma métrica = UM nó; Flawless lê-se `outcome →(=win)→ regra ←(≤0)← deaths`
   → `flawless_victory` → achievement; critério por métrica visível como
   edge com label.
2. Fluxo completo SEM drawer: nova regra → editar condições dentro do nó
   (clique em fonte pré-preenche) → "+" cria event type inline → salvar
   persiste / cancelar zera. Nó rascunho arrastável só pelo header.
3. Regra emitindo type já coberto por regra incondicional mostra o hint.
4. Event type órfão permite criar achievement dali; busca esmaece não-casados.
5. Toggle Stats (045) segue funcionando; `npm run build` verde.
