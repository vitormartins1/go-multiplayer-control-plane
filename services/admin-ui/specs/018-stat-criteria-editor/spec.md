# Spec: 018-stat-criteria-editor

**Serviço:** admin-ui
**Status:** Done (2026-07-06)
**Data:** 2026-07-06
**Parte de:** cauda da fatia A1 (`achievements/008-stat-criteria`, ADR-0023). Par de UI do critério por métrica.

## Contexto

O catálogo de conquistas (aba Achievements do jogo) só editava critério **por evento** (`eventType`). A fatia A1 adicionou o critério **por stat**. Esta cauda faz o editor (criar/editar) escolher entre os dois e enviar os campos certos.

## Comportamento

- THE SYSTEM SHALL, nos forms de criar/editar conquista, mostrar um seletor de **critério** (`SegmentedControl`: "Por evento" | "Por métrica"):
  - **evento** → campo `Event type` (obrigatório) + `Meta (ocorrências)`.
  - **métrica** → campo `Stat` (valida `^[a-z][a-z0-9_]*$` no cliente) + `Agregação` (`cumulative` = acumulado | `per_match` = melhor de uma partida) + `Meta (valor-alvo)`.
- Ao salvar, THE SYSTEM SHALL montar o payload zerando o lado que não se aplica (o backend rejeita evento+stat juntos). O servidor é a verdade da validação.
- Ao **editar** uma conquista existente, THE SYSTEM SHALL pré-selecionar o critério a partir de `statKey` (preenchido → métrica; vazio → evento).
- THE SYSTEM SHALL, na tabela do catálogo, mostrar o critério: `Badge` `stat · agregação` para as por métrica, ou o `eventType` para as por evento.

## Contratos de Interface (cliente)

- `src/lib/api.ts`: `CatalogAchievement`/`NewAchievement` +`statKey?`/`statMode?`; tipo `StatMode`.
- `src/pages/GameAchievements.tsx`: componente `CriterionFields` (reutilizado nos dois forms); `buildCriterion` monta o payload; validators condicionais ao critério; coluna "Critério".

## Casos de Teste / Verificação

- [ ] criar por métrica (`kills`/`per_match`/5) → salva; badge aparece na tabela
- [ ] alternar evento↔métrica mostra/esconde os campos certos
- [ ] `stat` inválido (maiúsculo/começa com dígito) → bloqueia no cliente
- [ ] editar conquista por stat pré-seleciona "Por métrica" com os valores
- [ ] `npm run build` (typecheck) limpo

## Fora do escopo

- `teamId` (follow-up da A1); importar critério por stat via CSV (mantém só evento no template por ora).
