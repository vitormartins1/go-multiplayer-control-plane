# Spec 046 — Vista de Pipeline do fan-out (aba Analytics)

**Status:** Approved (aprovada em conversa 2026-07-09, junto com o adendo do
ADR-0028 e a spec achievements/013)
**Serviço:** admin-ui (read-only; dados das APIs já consumidas)
**Relaciona:** ADR-0028 (adendo — decisão de fronteira entre consumers),
ADR-0020/0021 (fan-out), ADR-0024 (modos), specs 043–045 (canvas kit)

## Contexto

As métricas (`scoringStats`) e o fluxo de dados do jogo ficam escondidos no
form do modo; e a decisão de fronteira do adendo ADR-0028 (event rules são só
do achievements; leaderboard/analytics traduzem o envelope bruto) existe só em
texto. A vista de pipeline é o **mapa de dados do jogo**: mostra o envelope da
sessão se ramificando pros três consumers e o que cada um produz — 3º uso do
kit flow.

## Comportamento

- A aba **Analytics** do jogo ganha um toggle **Números | Pipeline**.
- A vista Pipeline é um canvas **read-only** (sem paleta/criação) de 3 camadas:
  1. **Modos** do jogo — card por modo: `modeId`, arquétipo, chips das
     `scoringStats` declaradas (ou "sem stats" — caso host_world_dropin).
  2. **Envelopes** — um nó por kind produzido pelos modos do jogo
     (queue_arena→`win_loss_draw`, host_world_dropin→`participation_event`,
     battle_royale→`placement`, coop_mission→`collective`).
  3. **Consumers** — 3 cards fixos com o que cada um produz DESTE jogo:
     - **achievements**: nº de regras efetivas, event types e achievements
       (por evento + por stat);
     - **leaderboard**: boards (score + um por stat declarada);
     - **analytics**: agregação bruta (auditoria — pré-política).
- **Edges honestas**: modo → seu kind; kind → só os consumers que o tratam.
  `participation_event` **não** liga ao leaderboard (o consumer descarta —
  a fronteira do adendo fica visível, não escondida em texto).
- Nós clicáveis abrem o painel flutuante com detalhe (mesmo padrão da 044);
  o card do achievements aponta pra aba Event Rules como "onde as regras
  moram".
- Jogo sem modos: estado vazio com orientação (cadastrar um modo).

## Fora de escopo

- Drift/validações extras (a 045 já cobre stat×achievement; boards são
  automáticos por stat — nada a validar aqui por ora).
- Números de tráfego por edge (contadores) — evolução futura se houver
  demanda.
- Qualquer mudança de backend.

## Critérios de aceite

1. Toggle alterna Números/Pipeline sem reload; a vista Números permanece
   intacta.
2. O grafo reflete só os modos/kinds reais do jogo; `participation_event` sem
   edge pro leaderboard.
3. Conteúdo dos cards derivado 100% das APIs (modos, rules, catálogo) — nada
   hardcoded além do mapeamento arquétipo→kind e kind→consumers (que espelha
   o dispatch real dos consumers Go).
4. `npm run build` verde; sem dependência nova.
