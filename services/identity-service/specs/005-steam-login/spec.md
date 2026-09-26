# Spec: 005-steam-login

**Serviço:** identity-service
**Status:** Done (validação com a Steam real depende da publisher key do Steamworks do jogo)
**Data:** 2026-09-26
**Decisão:** rascunho A de `docs/review/ESTRATEGIA-PRODUTO-2026-09.md` (identidade federada, Steam primeiro)

## Contexto

O Folclore Souls vai para a Steam. Hoje o identity só tem conta com senha e login anônimo — nenhum jogador real
vai criar uma senha num Souls-like da Steam. O caminho padrão é o cliente pegar um ticket da Steam
(`ISteamUser::GetAuthTicketForWebApi("<identidade>")`) e o backend validá-lo na Web API da Steam
(`ISteamUserAuth/AuthenticateUserTicket`), recebendo o SteamID64. O resto do control plane não muda: continua
recebendo o JWT de sempre.

## Comportamento

- `POST /sessions/{provider}` com `{"ticket": "<hex>", "gameId": "<opcional>"}`; hoje `provider = steam`.
- WHEN o ticket é válido -> THE SYSTEM SHALL achar a conta vinculada ao SteamID (tabela `external_identities`) ou,
  na primeira vez, **criar a conta** (handle `steam_<14 últimos dígitos do SteamID>`, sem senha) e vincular; emitir
  o JWT de sempre (mesmo `iss`/`aud`, claim extra `idp: "steam"`) e marcar presença no jogo se `gameId` vier
  (ADR-0019).
- Erros: provedor desconhecido ou não configurado → 404 `provider_not_supported`; ticket malformado ou recusado pela
  Steam → 401 `invalid_ticket`; jogador com `publisherbanned` → 403 `banned`; Steam fora do ar ou chave recusada →
  503 `provider_unavailable`.
- Conta criada pela Steam não tem senha: `POST /sessions` com handle/senha nunca autentica nela.
- Configuração (env): `STEAM_WEB_API_KEY` (publisher key), `STEAM_APP_ID`, `STEAM_TICKET_IDENTITY` (padrão
  `control-plane`; o cliente precisa pedir o ticket com a mesma identidade), `STEAM_API_BASE_URL` (padrão
  `https://partner.steam-api.com`). Sem chave ou app id, o provedor não é registrado (404).

## Critérios de aceite

1. Service: primeira vez cria e vincula; segunda vez reutiliza a conta; corrida na criação (vínculo já existe)
   relê a conta; handle ocupado tenta um alternativo; banido e ticket inválido propagam (mocks).
2. Adapter Steam contra um servidor fake (httptest): OK, erro da Steam, publisher ban, 5xx/403, ticket não hex.
3. Repositório (integração, Postgres real): cria conta + vínculo numa transação; acha pelo vínculo.
4. Transport: códigos HTTP acima.
5. Validado ao vivo com um servidor Steam falso: login → JWT aceito pelo matchmaking/achievements.

## Fora de escopo

GOG/EOS (mesmo desenho de provedor, quando o jogo for para lá); vincular Steam a uma conta com senha existente;
espelhar conquistas na Steam; nome de perfil via `GetPlayerSummaries`.
