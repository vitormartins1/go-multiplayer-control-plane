# Spec 049 — Agrupamento da coluna de fontes + edges de métrica discretas

> Status: **draft** (aguardando aprovação)
> Fatia: pequena/aditiva (só layout e estilo de edge; zero backend, zero
> mudança de dados) — spec curta, sem plan/tasks separados.

## Contexto / problema

Feedback do Vitor testando a 048 no Neon Breach: a coluna 1 (fontes) mistura
facts estruturais (`outcome`, `role`, `event`), `mode` e stats (`stat.*`) numa
ordem ditada só pelo baricentro (conectividade), então os tipos se intercalam
e a coluna fica confusa — "não sei se o stat na mesma coluna do fact fica
legal".

Diagnóstico: **não** é limite do React Flow nem da escala de dados; é a nossa
ordenação. Dois agravantes:

1. O baricentro intercala kinds — um stat pode aparecer entre dois facts.
2. As edges de métrica acumulada (`stat.x → achievement`, coluna 1 → coluna 4)
   cruzam o canvas inteiro com peso visual igual ao das edges de condição,
   "amarrando" visualmente os stats a tudo.

Manter o modelo unificado da 048 (um nó por chave; condição = label de edge) —
ele está semanticamente certo e foi aprovado. O ajuste é só de hierarquia
visual.

## Comportamento (EARS)

1. **Enquanto** o canvas de Event Rules estiver montado, a coluna de fontes
   DEVE ser ordenada em três grupos fixos, de cima para baixo:
   **facts estruturais** → **mode** → **stats** (`stat.*`).
2. **Dentro de cada grupo**, a ordenação DEVE continuar por baricentro
   (reduzir cruzamentos), e stats sem nenhuma edge continuam afundando para o
   fim do próprio grupo.
3. **Entre grupos**, DEVE haver separação visual: gap vertical maior
   (~2× GAP_Y) e um rótulo de seção discreto acima de cada grupo
   ("FACTS DA PARTIDA", "MODO", "MÉTRICAS"), não interativo e não
   selecionável. O rótulo de um grupo vazio não aparece.
4. **Quando** um achievement por métrica existir, o critério DEVE aparecer
   como texto no próprio nó do achievement (ex.: `kills acum. ≥ 500`, junto
   ao badge MÉTRICA), de modo que a informação não dependa da edge.
5. **Enquanto** nada estiver selecionado, as edges de métrica acumulada
   (`stat → achievement`) DEVEM ser discretas (esmaecidas + tracejadas);
   **quando** a fonte ou o achievement da edge estiver selecionado/no caminho
   destacado, a edge DEVE acender com o estilo pleno atual (animada).
6. As edges de condição (fonte → regra) e o destaque de caminho ao clicar
   (highlight bidirecional) NÃO mudam.
7. O rascunho in-canvas (048) NÃO muda: nó de rascunho, ghosts e pré-registro
   de condição ao clicar numa fonte continuam como estão.

## Critérios de aceite

- [ ] Neon Breach: coluna 1 mostra `outcome`/`role`/`event`/`placement` no
      topo, `mode` no meio, `stat.*` embaixo, com rótulos de seção.
- [ ] Edge `revives → (acum. ≥ 15) → Médico de Campo` nasce esmaecida;
      clicar em `revives` ou no achievement acende a edge.
- [ ] Nó do Exterminador exibe `kills acum. ≥ 500` no corpo.
- [ ] Clicar em `revives` continua destacando as DUAS vias (condição da regra
      Mestre Curativo + critério acumulado do Médico de Campo).
- [ ] Busca, criação in-canvas, hint de redundância e quick-create intactos.
- [ ] Build verde + smoke visual Playwright (screenshot da coluna agrupada).

## Fora de escopo (registrado, não fazer)

- Colapsar stats não usados num nó único expansível — só se a poluição
  persistir depois do agrupamento.
- Modo fullscreen do canvas (ideia já registrada).
- V2b (supressão de regra global por jogo) — próxima fatia do motor.
