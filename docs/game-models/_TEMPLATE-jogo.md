# Ficha: <Nome do jogo / mecânica>

**Ficha nº:** NN · **Data:** AAAA-MM-DD · **Status:** rascunho | analisado

> Copie este arquivo para `NN-<slug>.md` e preencha. O objetivo não é descrever o jogo por completo,
> e sim fazer **engenharia reversa** do que o backend precisaria para dar suporte a esse formato.

## 1. Visão geral

- **O que é (1 parágrafo):**
- **Jogos de referência (reais):** <ex.: Rocket League, Among Us, Slay the Spire multiplayer…>
- **Por que nos interessa:** <o que este jogo estressa no backend que os anteriores não estressaram>

## 2. Classificação (eixos)

Preencha usando os eixos do [README](./README.md#eixos-de-classificação-o-vocabulário-comum).

| Eixo | Valor |
|---|---|
| Estrutura social | |
| Condição de término | |
| Modelo de resultado | |
| Granularidade do resultado | |
| Persistência competitiva | |
| Tamanho & composição | |
| Sincronicidade | |
| Ciclo de vida do jogador | |
| Efeito do abandono | |

## 3. Ciclo de vida da sessão

Descreva do "juntou gente" ao "acabou". Use Mermaid se ajudar (estados/sequência).

- **Como começa:** <matchmaking? lobby? convite?>
- **Enquanto roda:** <o que o servidor autoritativo controla; join-in-progress? espectador?>
- **Como termina:** <gatilho concreto do término>
- **Quem/o quê reporta o resultado:** <game server? host? consenso?>

## 4. Matchmaking

- **Como se forma a partida:** <fila? lobby manual? party?>
- **Composição:** <NvN? FFA de N? assimétrico?>
- **Critérios:** <skill/MMR? região? modo? nada (FIFO)?>
- **O que precisa viajar no ticket:** <além de `game_id`>

## 5. Resultado da partida

O ponto mais importante para o backend. Seja concreto.

- **O que define vitória/derrota/empate/colocação:**
- **É por jogador, por time ou coletivo?**
- **Estrutura do resultado que o game server enviaria:** <esboce o payload conceitual>
- **Efeito pós-partida:** <XP? conquistas? ranking/MMR? nada?>
- **Casos de borda:** empate · abandono/forfeit · desconexão · partida anulada · sem vencedor

## 6. Impacto no backend (o elo com o control plane)

Confronte com o [Estado atual do backend](./README.md#estado-atual-do-backend-baseline).

- **Cabe no modelo atual?** ✅ / ⚠️ / ❌ — justifique.
- **game-registry:** <precisa de novo metadado de jogo/modo?>
- **matchmaking:** <precisa de times/skill/modo?>
- **session:** <precisa de times na sessão? novo formato de `ReportResult`?>
- **resultado & fan-out:** <como o `PlayerResult`/eventos mudariam?>
- **Gaps para o Log de generalizações:** <liste; serão levados ao README>

## 7. Demo mínima (validação)

O corte mais simples que prova que o backend suporta este formato (não o jogo inteiro).

- **Escopo da demo:**
- **O que ela valida no backend:**
