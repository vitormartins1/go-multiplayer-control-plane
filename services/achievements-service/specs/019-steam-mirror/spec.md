# Spec: 019-steam-mirror

**Serviço:** achievements-service (+ identity `006`, admin-bff, admin-ui)
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** roadmap da estratégia de produto 2026-09, item 3 (aprovado pelo dono)

## Contexto

O Folclore Souls vai lançar na Steam. O jogador espera ver a conquista na Steam; o operador quer o motor de regras
daqui (event-rules, stats, console). A decisão da estratégia: o achievements **continua a fonte da verdade** e
**espelha** o desbloqueio na Steam.

## Comportamento (EARS)

1. Uma conquista do catálogo **deve** aceitar um campo opcional `steamApiName` (o "API Name" cadastrado no
   Steamworks). Vazio = não espelha.
2. **Quando** um jogador desbloquear uma conquista com `steamApiName` do jogo configurado para a Steam
   (`STEAM_GAME_ID`), o sistema **deve** marcar o desbloqueio na Steam do jogador
   (`ISteamUserStats/SetUserStatsForGame`, `name = steamApiName`, `value = 1`) — de forma **assíncrona**: o
   desbloqueio aqui nunca espera nem falha por causa da Steam.
3. O SteamID do jogador **deve** vir do vínculo do identity (`GET /admin/players/{id}/external-identities`, spec
   `identity/006`). **Se** o jogador não tiver conta Steam vinculada (login por senha/anônimo), o desbloqueio **deve**
   ser marcado como `skipped` e não voltar a ser tentado.
4. **Se** a Steam ou o identity falharem (rede, 5xx, resultado ≠ 1), o sistema **deve** tentar de novo com espera
   crescente (1 min, 2 min, 4 min… até 1 h) e registrar o último erro. Depois de 10 tentativas, `failed` (volta a ser
   tentado só se o operador limpar o estado).
5. **Quando** o operador preencher `steamApiName` numa conquista que jogadores já tinham desbloqueado, esses
   desbloqueios **devem** ser espelhados também (backfill natural: o pendente é "desbloqueado e não espelhado").
6. Sem `STEAM_WEB_API_KEY`/`STEAM_APP_ID`/`STEAM_GAME_ID`/`IDENTITY_URL` o espelhamento fica **desligado** (log no
   boot), sem afetar o resto.
7. Métrica `achievements_steam_sync_total{result}` (`synced`, `skipped`, `retry`, `failed`).
8. O console **deve** permitir cadastrar o `steamApiName` no formulário da conquista e mostrar um selo "Steam" no
   catálogo.

## Fora de escopo

Stats da Steam (`SetUserStatsForGame` com stats numéricos), remover conquista na Steam, mais de um app Steam por
deploy (hoje um `STEAM_APP_ID` ↔ um `STEAM_GAME_ID`, igual ao login), GOG/EOS.

## Critérios de aceite

- Unit (service do espelhamento, mocks): sincroniza, pula sem Steam, reagenda com backoff, falha após o limite.
- Adapter Steam contra servidor falso (httptest): sucesso (`result: 1`), resultado ≠ 1, 5xx, 403.
- Integração (Postgres real): a fila de pendentes pega só desbloqueio com `steamApiName` do jogo configurado, sem
  espelhamento, com `next_attempt` vencido; marcar estados.
- Ao vivo com Steam falsa: desbloqueio na demo → a Steam falsa recebe o `SetUserStatsForGame` do SteamID certo.
