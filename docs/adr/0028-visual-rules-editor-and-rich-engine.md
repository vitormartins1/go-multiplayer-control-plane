# ADR-0028: Editor visual de regras (canvas low-code) e roadmap do motor de regras rico

**Status:** Accepted
**Data:** 2026-07-09
**Serviço(s) afetado(s):** admin-ui, achievements-service
**Relaciona:** ADR-0027 (motor de regras data-driven — este ADR **reabre deliberadamente** a Opção C lá descartada; ver adendo naquele ADR), ADR-0023 (outcome polimórfico por arquétipo), spec `admin-ui/043-visual-rules-editor`

---

## Contexto

O ADR-0027 tornou o mapeamento evento→achievement-event-type data-driven, mas
descartou (Opção C) o motor de regras genérico — comparadores, ranges,
operadores lógicos — por falta de evidência na época. O projeto maturou desde
então: os 4 arquétipos do ADR-0023 estão completos no control plane
(host_world_dropin inclusive com contrato cross-repo), e a tese do produto
evoluiu — o objetivo agora é deixar o control plane **genérico o suficiente
para regras avançadas** de integração control plane ↔ data plane. Isso é uma
decisão de direção do produto, não especulação técnica: a limitação do motor
atual (match por igualdade exata + união) já tem um caso concreto de
insuficiência conhecido (o limiar `top_n` do battle_royale, hardcoded na
normalização como `BattleRoyaleTopNThreshold`).

No lado da edição, a aba Event Rules atual (tabela + modal de pares
chave/valor) não comunica o efeito combinado das regras nem escala em
expressividade quando o motor crescer. A inspiração de UX é o exemplo *Call
Center* do Foblex Flow (trigger → condição → ação, paleta lateral, nós ricos)
— mas o Foblex Flow é uma biblioteca **Angular**, incompatível com o admin-ui
(React 18 + Mantine + Vite).

## Alternativas Consideradas

### Lib do canvas

- **Foblex Flow** — a inspiração original; Angular-only. Descartada como lib;
  permanece como referência de UX.
- **Rete.js** — motor de dataflow completo (processamento nos nós); mais
  pesado que o necessário — nosso avaliador roda no backend, o canvas é
  edição/visualização, não execução.
- **JointJS / GoJS / jsPlumb Toolkit** — maduras, mas comerciais/pagas;
  injustificável havendo alternativa MIT já instalada.
- **React Flow (`@xyflow/react`)** *(escolhida)* — já é dependência do projeto
  (TopologyGraph); cada nó é um componente React comum (Mantine dentro do nó,
  tema do console); ecossistema maduro com template oficial de workflow editor
  (paleta drag-and-drop + auto-layout). A simplicidade visual do TopologyGraph
  atual é consequência de usar os nós *default* da lib, não um limite dela.

### Evolução do motor

- **Status quo para sempre** (subset-match + união) — barato, mas trava a tese
  de genericidade e mantém parâmetros de arquétipo hardcoded em Go.
- **DSL completa** (CEL, expr, JsonLogic) — poder máximo, mas o admin viraria
  programação textual (contra a proposta low-code) e a indireção/debuggability
  piora muito de uma vez.
- **Extensão incremental do JSONB** *(escolhida)* — o valor de cada chave do
  `match` passa a aceitar **string** (igualdade — formato atual, compat total)
  **ou objeto** `{"op": "lte|gte|ne|in|...", "value": ...}`. O avaliador
  continua uma única função pura e testável; operador desconhecido = erro
  explícito, nunca match silencioso.

## Decisão

1. **Canvas:** React Flow com kit próprio de nós tematizados
   (`services/admin-ui/src/components/flow/`), UX no padrão call-center
   (paleta lateral, painel de propriedades, edges animadas destacando o
   caminho fact → regra → event type → achievement). A fase 1 (spec
   `admin-ui/043-visual-rules-editor`) é UI-only sobre o CRUD existente do
   ADR-0027 — **honestidade visual**: o canvas só desenha capacidade que o
   motor já tem.
2. **Motor V2 — roadmap comprometido, nesta ordem, cada etapa com spec própria
   no achievements-service:**
   - **V2a — comparadores:** operadores numéricos/de conjunto no match.
     Elimina o fact derivado `tier=top_n` hardcoded — `placement: {"op":
     "lte", "value": 10}` vira regra editável no admin.
   - **V2b — supressão:** um jogo pode **desabilitar** uma regra global para
     si (a pergunta aberta registrada no ADR-0027; hoje só soma, nunca
     suprime).
   - **V2c — prioridade/exclusividade:** first-match-wins opcional por grupo —
     só se casos reais pedirem (única etapa ainda condicionada a evidência).
3. **Compatibilidade:** regras existentes permanecem válidas sem migração de
   dados (valor string = igualdade). O canvas ganha os nós/controles de cada
   V2x junto com a etapa correspondente, nunca antes.

## Consequências

### Positivas
- Caminho claro para a genericidade sem big-bang: cada V2x é uma fatia SDD
  independente, com o formato de dado já pensado para não quebrar o anterior.
- Kit de canvas reutilizável — candidatos já identificados: upgrade do
  TopologyGraph, visualização do fan-out de sessão, futuros brackets de
  matchmaking (regra de três: extrair abstração comum no terceiro uso).
- Admin low-code de verdade: vocabulário E lógica condicional editáveis sem
  redeploy, alinhado ao princípio "novos jogos entram por cadastro, não por
  código" (ADR-0023).

### Negativas / Trade-offs
- O avaliador cresce (de ~20 linhas para estimadas ~80 + tabela de
  operadores); mais superfície de teste e de erro de configuração pelo admin.
- O front precisa validar e renderizar operadores (não só pares texto=texto).
- Supersede parcialmente a justificativa da Opção C do ADR-0027 — registrado
  como adendo lá, apontando para cá.

### Neutras / Implicações
- `BattleRoyaleTopNThreshold` só sai do código Go quando V2a chegar; até lá o
  fact derivado `tier` continua como está.
- A visão em tabela da aba Event Rules permanece como view alternativa do
  mesmo dado (toggle Tabela/Canvas).

## Adendo (2026-07-09) — dimensão de FACTS e fronteira entre consumers

Revisão do pipeline (session publisher → envelopes → consumers) a pedido do
usuário, após constatar que as seeds esgotam o espaço útil de regras nos
arquétipos de 1 fact (coop/queue: só `outcome`). Diagnóstico: o motor é
genérico, mas o **vocabulário de facts** é o gargalo. O roadmap V2 ganha a
dimensão "facts" (dado), complementar à dimensão "motor" (V2a–V2c):

1. **F1 — fact `mode`** (zero mudança de contrato): `modeId` já viaja em TODOS
   os envelopes (ADR-0024, aditivo) e o consumer do achievements o ignorava.
   Passa a virar fact (`mode=<modeId>`) — destrava regras por modo (ex.:
   "vitória no ranked"). Spec `achievements-service/013-mode-fact`.
2. **Stats-as-facts** (depende de V2a): stats por partida como facts numéricos
   com comparadores — destrava condição composta (ex.: *flawless victory* =
   `outcome=win` + `deaths <= 0`), inexpressável hoje.
3. **Mission/difficulty** (contrato cross-repo): os catálogos do perfil do
   coop são só declaração — o session-service não sabe qual missão/dificuldade
   foi jogada (o resultado gRPC não carrega). Facts desse tipo exigem o data
   plane reportar no resultado (aditivo). Adiado até a fatia cross-repo.

**Decisão de fronteira (registrada para não re-litigar):** as event rules são
política **exclusiva** do contexto achievements. Leaderboard e analytics
continuam traduzindo o envelope bruto para as línguas deles (boards/agregados)
— compartilhar a tabela de regras vazaria política entre contextos (um rename
de event type de conquista mudaria rankings) e corromperia o valor de auditoria
do analytics (regra muda com o tempo; história não pode mudar junto). O que se
reutiliza entre consumers é o **contrato** (envelope), o **padrão** (política
data-driven por serviço — ex. futuro: `scoring_rules` do leaderboard para a
política placement→pontos, hoje hardcoded) e a **ferramenta** (canvas kit; a
vista de pipeline — spec `admin-ui/046` — torna a fronteira visível).

## Referências
- Spec da fase 1: `services/admin-ui/specs/043-visual-rules-editor/spec.md`.
- React Flow: https://reactflow.dev/ — template Workflow Editor
  (https://reactflow.dev/ui/templates/workflow-editor) e exemplo drag-and-drop.
- Referência de UX: https://flow.foblex.com/examples/call-center (Angular; só
  inspiração visual).
