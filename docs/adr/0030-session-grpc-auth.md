# ADR-0030: Autenticação no gRPC do session — jogador por JWT, matchmaking por internal key, game server por chave de servidor

**Status:** Accepted (decisão D1 da revisão de produto 2026-09, aprovada pelo dono em 2026-09-26)
**Data:** 2026-09-26
**Serviço(s) afetado(s):** session-service, matchmaking-service, infra (compose, fleets Agones), data plane (Unity)
**Relaciona:** ADR-0018 (internal key serviço-a-serviço — esta ADR a estende ao gRPC do session), ADR-0010/0011 (connect token)

---

## Contexto

O gRPC do session-service não tinha autenticação nenhuma (`session-service/cmd/server/main.go`, sem interceptor) e a
porta é publicada (`50052`). Qualquer cliente que a alcançasse podia abrir mundos em nome de qualquer
`host_player_id` (alocando GameServers — custo), entrar em mundos, e reportar resultados e eventos que viram
conquista e ranking. O matchmaking já exige o JWT do jogador (`Invade`, sinais), mas chama o session sem credencial.
O jogo (Folclore Souls) já tem o JWT do identity em mãos e hoje extrai o `sub` localmente para montar o `OpenWorld`.

Há três tipos de chamador do session, com necessidades diferentes:

| Chamador | RPCs |
|---|---|
| **cliente do jogo** (jogador) | `OpenWorld`, `LeaveWorld`, `CloseWorld` |
| **matchmaking** (serviço interno) | `CreateSession`, `ListInvadableWorlds`, `JoinWorld` |
| **Dedicated Server** (data plane) | `ReportResult`, `ReportParticipationEvent`, `GetSessionConfig`, `LeaveWorld`, `CloseWorld` |

## Alternativas Consideradas

### Opção A: manter sem autenticação até a fase multiplayer do jogo voltar
- **Prós:** zero trabalho agora; zero risco de quebrar o jogo.
- **Contras:** o maior risco do produto fica aberto; qualquer deploy fora da máquina local fica inviável.

### Opção B: um token por GameServer emitido na alocação (annotation do Agones)
- **Prós:** credencial por sessão, revogada junto com o GameServer.
- **Contras:** exige ler annotations via SDK do Agones no Unity; não funciona com o allocator `static` de dev; mais
  código dos dois lados para um ganho pequeno no estágio atual.

### Opção C (escolhida): credencial por tipo de chamador + rollout `warn` → `enforce`
- **Jogador:** `authorization: Bearer <JWT do identity>` (mesmo `shared/auth.Verifier` do matchmaking). O host do
  `OpenWorld` é o `sub` do token; `host_player_id` no request, se vier, precisa bater.
- **Matchmaking:** metadata `x-internal-key` (a mesma chave da ADR-0018).
- **Dedicated Server:** metadata `x-game-server-key` com uma chave por deploy (`GAME_SERVER_KEY`), entregue às
  fleets por `Secret` do Kubernetes e ao compose por env. Funciona com o allocator `static` e com Agones.
- **Rollout:** `SESSION_AUTH_MODE=off|warn|enforce`. Em `warn` (padrão enquanto o jogo não é atualizado) toda
  falha é logada e contada (`session_auth_rejections_total`) mas a chamada segue com a semântica antiga; em
  `enforce` a falha vira `Unauthenticated`/`PermissionDenied`.
- **Prós:** fecha o buraco com pouco código, sem quebrar o jogo no dia do deploy; o dono vira a chave quando o
  cliente e o server do jogo mandarem as credenciais; mede quem ainda chama sem credencial antes de virar.
- **Contras:** a chave do game server é compartilhada por deploy (vazou, troca em todas as fleets); em `warn` o
  buraco continua aberto, só visível.

## Decisão

Opção C. A autorização fina ("só o host fecha o mundo", "só o host invoca sinal no próprio mundo") mora no service;
o interceptor só identifica o chamador. `JoinWorldRequest` ganha `summoner_player_id` (opcional): quando o
matchmaking injeta um co-op via `SummonSign`, manda o `sub` de quem invocou, e o session exige que seja o host do
mundo — fecha a lacuna em que qualquer jogador autenticado invocava sinais no mundo de outro.

## Consequências

### Positivas
- Nenhum RPC do session aceita chamada anônima em `enforce`.
- O jogo deixa de precisar extrair o `sub` do JWT: o `OpenWorld` pode mandar `host_player_id` vazio.
- `session_auth_rejections_total{method,reason}` mostra, antes de virar para `enforce`, quem ainda não se adaptou.

### Negativas / Trade-offs
- Mais uma chave para operar (`GAME_SERVER_KEY`); rotação exige atualizar o `Secret` e reciclar as fleets.
- Em topologia P2P (host como autoridade, se um dia adotada) o "game server" é o cliente do host: a chave de
  servidor não pode ir num cliente. Essa topologia vai precisar de outra credencial (ex.: token de sessão emitido
  no `OpenWorld` e amarrado ao host) — decidir junto com a ADR dela.

### Neutras / Implicações
- Contrato: MINOR (1.2.0) enquanto o padrão é `warn`; virar o padrão para `enforce` é MAJOR para o jogo e fica
  registrado no `docs/contracts/CHANGELOG.md`.
