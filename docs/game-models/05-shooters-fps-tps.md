# Ficha: Shooters (FPS/TPS) — gênero ≠ arquétipo

**Ficha nº:** 05 · **Data:** 2026-07-06 · **Status:** analisado

> Jogos de tiro em 1ª e 3ª pessoa. A conclusão importante é de **meta-nível**: FPS/TPS **não** é um
> arquétipo — é o `queue_arena` (ficha 02) com **modos** e **scoring** configuráveis. E FPS vs TPS é **só câmera**.

## 1. Visão geral

- **O que é:** combate por armas, competitivo, quase sempre **por times**, em sessão fechada. A câmera (1ª ou 3ª pessoa) é a diferença cosmética; a **forma** do multiplayer é a mesma.
- **Jogos de referência (reais):**
  - **FPS:** Call of Duty, Battlefield, Counter-Strike 2, Valorant, Halo.
  - **TPS:** The Last of Us Factions (4v4, stealth/letalidade alta; servers off 2019, o standalone *The Last of Us Online* foi **cancelado** em 2023), Metal Gear Solid Δ: Snake Eater **Fox Hunt** (out/2025, até 12 players, stealth "hide-and-seek": modos *Survival Capture* e *Survival Intrude*).
- **Por que nos interessa:** o Vitor perguntou se vale um arquétipo para FPS e outro para TPS. **Não** — e ao investigar por quê, fica claro o princípio que organiza a taxonomia: **arquétipo é sobre a forma da sessão/matchmaking/resultado, não sobre o gênero/tema.**

## 2. Classificação (eixos)

| Eixo | Valor |
|---|---|
| Estrutura social | quase sempre **times NvN** (4v4, 5v5 … até 12v12/32v32); às vezes FFA |
| Condição de término | **modo-dependente:** kills-alvo/tempo (TDM) · território (Domination/Conquest) · **rounds** attack/defense (S&D, CS, Valorant) · objetivo (CTF, Fox Hunt) |
| Modelo de resultado | vitória/derrota **por time** (+ empate); igual ao `queue_arena` |
| Granularidade do resultado | por time (desfecho) + **stats por jogador** (kills, assists, objetivos, stealth) |
| Persistência competitiva | casual **ou** ranked/MMR (CS/Valorant são ranqueados) |
| Tamanho & composição | 4v4/5v5 (CS, Valorant, TLOU, MGS) → 12v12/32v32 (Battlefield) |
| Sincronicidade | tempo real, **dedicated server** (norma competitiva). ✅ nossa topologia |
| Ciclo de vida do jogador | respawn contínuo (TDM/Domination) **ou** sem respawn no round (S&D) |
| Efeito do abandono | derrota individual / bot ou vaga aberta; a partida segue |

## 3. FPS vs TPS — a resposta

**Para o control plane, são idênticos.** Matchmaking, sessão e resultado **não sabem nem se importam** com a câmera. A câmera (1ª/3ª pessoa) é **100% data plane** (Unity): input, render, hitscan/projétil, feel. Nada nela toca matchmaking, formação de times, condição de vitória ou formato de resultado.

> **Decisão: um único arquétipo cobre FPS e TPS.** Diferenciar seria criar dois nomes para a mesma forma de backend. O que varia entre jogos de tiro **não é a câmera** — é o **modo** (regras/vitória) e o **scoring**, que são parâmetros, não arquétipos.

## 4. O princípio: gênero ≠ arquétipo

Esta ficha não adiciona um arquétipo — **reforça o `queue_arena`** (ficha 02) e estabelece a regra:

- **Arquétipo** = forma da sessão/matchmaking/resultado (o que o backend modela).
- **Gênero/tema** (shooter, esporte, luta, MOBA…) = **conteúdo do data plane**.

Vários gêneros colapsam no **mesmo** `queue_arena`: um shooter competitivo, um MOBA 5v5, um esporte (Rocket League), um fighting 1v1 — todos são "N jogadores/times numa sessão fechada com um desfecho". O que os distingue no backend são **modo + scoring + tamanho**, tudo **configurável**. Isso é exatamente o "genérico dentro de um tipo" que o Vitor quer: o *tipo* é `queue_arena`, e o shooter é uma configuração rica dele.

## 5. O que os shooters *acrescentam* ao `queue_arena`

Três refinamentos (parâmetros, não arquétipos novos):

1. **Modo de partida** como preset de `{estrutura social, condição de vitória, granularidade}`:
   | Modo | Social | Vitória | Granularidade |
   |---|---|---|---|
   | Team Deathmatch | NvN | kills-alvo / maior no tempo | time |
   | Free-for-all | FFA | kills-alvo | jogador |
   | Domination/Conquest | NvN | controle de território/tickets | time |
   | Search & Destroy / CS | NvN | **melhor de N rounds** (sem respawn no round) | time |
   | CTF / Fox Hunt | NvN | objetivo (bandeiras/sapos/fases) | time |

2. **Sub-eixo de *rounds*** (novo detalhe de término): a partida pode ser **melhor de N rounds** com troca de lado e sem respawn dentro do round (CS/Valorant/S&D). O `queue_arena` precisa de um `matchStructure = single | best_of_rounds(N)`.

3. **Scoring rico / stats estruturadas no resultado:** o desfecho é do time, mas o resultado carrega **métricas por jogador** — kills, assists, objetivos, e **stealth/detecção** (MGS Fox Hunt e TLOU pontuam por *não ser visto*, não por kills). O **cálculo** é data plane; o que o **control plane recebe** deixa de ser `score int64` e vira um **mapa de stats** por jogador (alimenta leaderboard e achievements ricos: K/D, MVP, "fantasma").

## 6. Impacto no backend (o elo com o control plane)

Veredito: **✅ cabe — estende o `queue_arena`**, não cria arquétipo. (Primeira ficha ✅.)

- **game-registry:** `queue_arena` ganha params: `mode` (preset acima), `matchStructure` (single / best-of-N rounds), `teamSize`, `respawn` (contínuo / sem-respawn-no-round), `scoringStats[]` (quais métricas o resultado carrega). **Sem** campo de câmera — irrelevante ao backend.
- **matchmaking:** formar **times** (gap já aberto do baseline) + **ranked/MMR** opcional. Nada específico de "shooter".
- **session:** o `queue_arena` já cobre; rounds são majoritariamente gameplay (o server só reporta o placar de rounds no fim). Tamanhos grandes (Battlefield) = tuning de capacidade.
- **resultado & fan-out:** **stats estruturadas por jogador** no `PlayerResult` (não só `score`) + desfecho por time — reforça o **gap central** (resultado rico) e o alimenta com o caso mais concreto até aqui (leaderboards de K/D, MVP, objetivos).
- **Gaps para o Log:** `mode` como preset no perfil; `matchStructure` (best-of-N rounds); **stats estruturadas no resultado** (mapa de métricas por jogador, além de `score`).

## 7. Demo mínima (validação)

- **Escopo:** o mesmo veículo de arena (ficha 02), rodado como **TDM 2v2** (respawn, first-to-X kills, resultado por time) e depois como **best-of-3 rounds** (sem respawn no round) — só trocando `mode`/`matchStructure` no perfil. O resultado carrega um **mapa de stats** por jogador (kills, um objetivo).
- **O que valida:** (1) modo/estrutura vindos do perfil; (2) resultado por time + **stats estruturadas** por jogador; (3) que FPS/TPS não exige nada além do `queue_arena` (a câmera nunca aparece no backend).
