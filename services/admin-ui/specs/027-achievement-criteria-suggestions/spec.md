# Spec: 027-achievement-criteria-suggestions

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-07)
**Data:** 2026-07-07
**Parte de:** lote console-polish. Decisão de design: **sugestão, não validação dura** — o
envelope de stats é aberto por design (ADR-0023, params declarativos); o console sugere o
que o sistema já conhece, mas o texto livre permanece.

## Contexto

No form de conquista, `statKey` e `eventType` são TextInput livres, mas a resposta certa já
está no sistema: os stats vêm dos `scoringStats` dos modos do jogo; os eventos são o
vocabulário fixo derivado pelos sinks por arquétipo (constantes do achievements-service).

## Comportamento

- WHEN o critério for "Por métrica" → THE SYSTEM SHALL trocar o TextInput de stat por um
  `Autocomplete` cujas sugestões são a união dos `scoringStats` de todos os modos do jogo
  (via `listMultiplayerModes`) + o intrínseco `score`; cada sugestão indica a origem
  (ex.: "kills — modo tdm"). Texto fora da lista continua aceito (regex atual mantida).
- WHEN o critério for "Por evento" → THE SYSTEM SHALL trocar o TextInput por um
  `Autocomplete` com o vocabulário fixo dos arquétipos presentes nos modos do jogo:
  - queue_arena: `match_played`, `match_won`, `match_lost`, `match_drawn`
  - battle_royale: `battle_royale_played`, `battle_royale_win`, `battle_royale_top_n`
  - coop_mission: `coop_mission_played`, `coop_mission_success`, `coop_mission_failed`
  - host_world_dropin: `world_joined`, `invasion_won`, `coop_boss_helped`
  Jogo sem modos → lista completa dos 4 arquétipos. Texto livre continua aceito.
- THE SYSTEM SHALL aplicar o mesmo componente nos forms de criar e editar (CriterionFields).
- Falha ao carregar os modos → campos degradam para texto livre sem sugestões (sem erro).

## Contratos de Interface (cliente)

- Tabela `EVENT_VOCABULARY: Record<archetype, string[]>` no cliente (espelho das constantes
  do achievements-service; comentário aponta os arquivos-fonte).
- `GameAchievements.tsx` carrega os modos uma vez (useAsync) e deriva `statSuggestions` /
  `eventSuggestions`; `CriterionFields` recebe as sugestões por prop.

## Casos de Teste / Verificação

- [x] jogo com modos tdm (kills, assists) e ranked (kills) → sugestões únicas com origem
- [x] eventos sugeridos só dos arquétipos cadastrados; sem modos → lista completa
- [x] valor digitado fora da lista é aceito e salvo (creatable)
- [x] modos indisponíveis → form funciona como hoje
- [x] `npm run build` limpo

## Fora do escopo

- Validação dura/enforcement de schema; check de consistência (badge "não declarado") —
  fatia futura; sugestões no import CSV.
