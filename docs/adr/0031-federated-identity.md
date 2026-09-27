# ADR-0031: Identidade federada no identity — ticket da loja vira conta + o JWT de sempre (Steam primeiro)

**Status:** Accepted (rascunho A da estratégia de produto 2026-09, aprovado pelo dono em 2026-09-26)
**Data:** 2026-09-26
**Serviço(s) afetado(s):** identity-service, data plane (cliente Unity)
**Relaciona:** ADR-0030 (o JWT emitido aqui é o bearer que o session aceita), `identity/005-steam-login`

---

## Contexto

O Folclore Souls vai lançar na Steam (talvez GOG e outras lojas de desktop depois). O identity só tinha conta com
senha e login anônimo: nenhum jogador de Steam cria senha num Souls-like, e sem identidade real o resto do backend
(conquistas, invasão, analytics por jogador) não tem a quem se referir. A loja já autentica o jogador; o que falta é
o backend confiar nessa autenticação e mapear o ID da loja para uma conta daqui.

## Alternativas Consideradas

### Opção A (escolhida): provedor por provedor no identity, Steam primeiro
- O cliente pega um ticket (`ISteamUser::GetAuthTicketForWebApi("<identidade>")`) e chama
  `POST /sessions/{provider}`; o identity valida na Web API do provedor (`ISteamUserAuth/AuthenticateUserTicket`),
  vincula o ID externo a uma conta (`external_identities(provider, external_id, account_id)`; a primeira vez cria a
  conta) e emite **o mesmo JWT** de sempre.
- **Prós:** nada muda nos outros serviços; cada loja nova é um verificador a mais atrás da mesma porta; banimento
  VAC/publisher e dono do app (Family Sharing) chegam no login.
- **Contras:** um verificador por loja; a publisher key vira segredo de deploy.

### Opção B: EOS Connect como identidade única entre lojas
- **Prós:** uma integração cobre Steam/GOG/Epic (`ProductUserId`), grátis.
- **Contras:** mais uma dependência de terceiro no caminho crítico do login para um jogo que começa só na Steam.

### Opção C: trocar o identity pelo Nakama (ou PlayFab)
- **Prós:** login social pronto.
- **Contras:** troca a stack e contradiz a tese do projeto (ADR-0008).

## Decisão

Opção A. O provedor é parte da rota; hoje só `steam` está registrado. EOS entra como mais um provedor só se houver
cross-store de verdade. Senha e anônimo continuam existindo (dev, testes, admin).

## Consequências

- **Positivas:** o backend fica utilizável com jogadores reais sem tocar em session/matchmaking/achievements; o
  `player_id` continua sendo o UUID da conta (o SteamID fica no vínculo, não espalhado pelos serviços).
- **Negativas / riscos:** a validação real depende da publisher key e do app id do Steamworks do jogo
  (`STEAM_WEB_API_KEY`, `STEAM_APP_ID`); sem elas o endpoint responde 404 (provedor não habilitado). Indisponibilidade
  da Web API da Steam derruba o login federado (503) — o JWT já emitido segue valendo até expirar.
- **Contrato:** aditivo (contrato 1.3.0, `docs/contracts/CHANGELOG.md`).
