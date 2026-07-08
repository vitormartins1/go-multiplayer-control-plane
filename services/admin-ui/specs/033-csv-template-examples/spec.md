# Spec: 033-csv-template-examples

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** lote console-avatars (carona no mesmo branch). Estende a 014 (import CSV)
e alinha com a 018 (critério por métrica).

## Contexto

O modelo CSV baixável tem 3 exemplos genéricos e **não cobre critério por métrica** —
o formato atual nem tem colunas para `statKey`/`statMode`, então tudo que a UI de
formulário faz (018) fica inacessível ao cadastro em lote. CSV não tem comentários
padronizados; a documentação viável é o próprio exemplo: linhas de amostra cobrindo
cada tipo de critério, com descrições autoexplicativas.

## Comportamento

- THE SYSTEM SHALL aceitar no CSV as colunas opcionais `statKey` e `statMode`
  (`cumulative` | `per_match`): presentes e preenchidas → critério por métrica
  (`eventType` deve ficar vazio na linha, senão erro de parse na linha); ausentes ou
  vazias → critério por evento (comportamento atual, retrocompatível).
- THE SYSTEM SHALL validar por linha: `statKey` no padrão `[a-z][a-z0-9_]*`;
  `statMode` obrigatório quando há `statKey`; exatamente um de `eventType`/`statKey`.
- THE SYSTEM SHALL gerar o modelo baixável com header completo e ~6 linhas de exemplo
  cobrindo: evento simples (target 1), evento acumulado (target N), evento de outro
  arquétipo (ex.: `battle_royale_win`), métrica cumulativa, métrica per_match e métrica
  `score` (intrínseca) — nomes/descrições em pt-BR explicando o que cada linha demonstra.
- (Ajuste na implementação: o fluxo de import não tem preview de linhas — o arquivo vai direto
  para o serviço após o parse; o critério importado aparece na tabela do catálogo, onde a 029
  já sinaliza drift. A linha de preview prevista no rascunho não se aplica.)

## Contratos de Interface (cliente)

- `lib/csv.ts`: `CSV_TEMPLATE` novo; `parseCSV` com colunas opcionais + validação
  cruzada; `ParsedRow` ganha `statKey?`/`statMode?`.
- `GameAchievements.tsx`: passa `statKey`/`statMode` ao `importAchievements` (o BFF já
  aceita — `NewAchievement` tem os campos desde a 018; sem mudança de backend).

## Casos de Teste / Verificação

- [x] CSV antigo (sem as colunas novas) importa igual (retrocompat)
- [x] linha com `statKey` + `statMode` vira conquista por métrica; com ambos
  `eventType` e `statKey` → erro apontando a linha
- [x] modelo baixado reimporta 100% limpo (payload misto verificado contra o BFF: 3/3 created)
- [x] `npm run build` limpo

## Fora do escopo

- Comentários no CSV (não padronizado); export do catálogo existente; validação de
  vocabulário no import (027/029 já sinalizam drift na tabela depois do import).
