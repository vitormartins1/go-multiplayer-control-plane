# Spec: 002-matchmaking-session-integration

**Serviço:** matchmaking-service
**Status:** Done (2026-06-29)
**Data:** 2026-06-29

## Contexto

Conecta o matchmaking ao session-service: ao formar um match, em vez de devolver um `connect_info` **mock** (001-matchmaking-core), o matchmaking chama o `CreateSession` do session (gRPC) para **alocar um game server** e obter **um connect token por jogador**, e propaga isso no stream do ticket. Fecha o fluxo de ponta a ponta "entrar na fila → receber server + token para conectar no Unity Dedicated Server" (ADR-0010).

O matchmaking passa a ser **cliente gRPC** de outro serviço interno (control plane → control plane). O session já existe e é estável (session-service/001-session-core); esta feature **não altera o session**, só o consome.

Decisões de escopo (ver "Fora do escopo"): sem auth serviço-a-serviço entre matchmaking e session no MVP (control plane interno, como na spec do session); falha de provisionamento **encerra o stream** do jogador (ele re-tenta), sem re-enfileiramento automático.

## Comportamento

### Provisionar a sessão ao formar o match
- WHEN houver `matchSize` tickets ativos na fila -> o sistema forma o match (mesmo `matchId` para todos) e provisiona uma sessão chamando `CreateSession{matchId, players[]}` no session-service.
- WHEN o provisionamento **sucede** -> emite a cada jogador `TicketUpdate{status: MATCHED, matchId, players[], server_address, connect_token}` e **encerra** o stream.
- THE SYSTEM SHALL entregar a cada jogador **apenas o seu próprio** connect token (o token emitido para o `sub` daquele jogador); o token de um jogador nunca é enviado a outro.
- THE SYSTEM SHALL usar o **mesmo** `server_address` (o do server alocado) para todos os jogadores do match.
- THE SYSTEM SHALL continuar formando e provisionando outros matches enquanto um provisionamento está em andamento (um `CreateSession` lento não bloqueia a fila).

### Falha de provisionamento
- IF o session-service não tem game server livre (responde `ResourceExhausted`) -> emite a cada jogador do match `TicketUpdate{status: FAILED, reason}` e **encerra** o stream.
- IF o `CreateSession` falha por qualquer outro motivo (session indisponível, erro de rede, timeout) -> emite a cada jogador `TicketUpdate{status: FAILED, reason}` e **encerra** o stream.
- THE SYSTEM SHALL, em caso de falha de provisionamento, **não** deixar os jogadores presos na fila nem em estado pendente — o ticket é descartado e o jogador é livre para chamar `FindMatch` de novo.
- THE SYSTEM SHALL tratar o `matchId` de um match cujo provisionamento falhou como descartado (não reaproveita o mesmo `matchId`).

### Inalterado em relação à 001
- Autenticação do jogador (JWT do identity via interceptor; `sub` identifica o jogador), entrada na fila com `SEARCHING`, formação FIFO por `matchSize`, um ticket por jogador, e cancelamento via `ctx` (sai da fila) permanecem como na 001-matchmaking-core.

## Contratos de Interface

### Protobuf (`Matchmaking`) — alterações
```
message TicketUpdate {
  Status status = 1;
  string match_id = 2;         // preenchido em MATCHED/FAILED
  repeated string players = 3; // playerIDs do match
  string server_address = 4;   // NOVO: endereço do game server alocado (MATCHED)
  string connect_token = 5;    // NOVO: connect token EdDSA do PRÓPRIO jogador (MATCHED)
  string reason = 6;           // NOVO: motivo quando FAILED
}

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_SEARCHING = 1;
  STATUS_MATCHED = 2;
  STATUS_FAILED = 3;           // NOVO: provisionamento da sessão falhou
}
```
- O campo `connect_info` (string mock da 001) é **removido**.

### SessionProvisioner (Port de saída — nova)
- `Provision(ctx, matchID string, players []string) (Provisioned, error)` — chama o `CreateSession` do session; devolve o endereço do server e o token de cada jogador. Erro `ErrNoServer` quando o session responde `ResourceExhausted`.

### Tipos (domínio)
- `Provisioned{ ServerAddress string; Tokens map[playerID]string }` — resultado do provisionamento (endereço do server + connect token por jogador).
- `Match` (existente) passa a carregar o resultado do provisionamento (server + tokens por jogador) em vez do `ConnectInfo` mock.

## Modelo de Estado

- A fila de tickets segue **em memória** sob `sync.Mutex` (inalterado, 001).
- O provisionamento é **I/O externo** (gRPC ao session): feito fora da seção crítica da formação, de modo a não bloquear o loop (ver "Comportamento"). O matchmaking não persiste sessões — quem é dono do estado da sessão é o session-service.

## Casos de Teste Obrigatórios

- [ ] `matchSize` jogadores entram -> provisionamento ok -> cada um recebe `MATCHED` com o **mesmo** `matchId`, o **mesmo** `server_address` e o `connect_token` **do próprio** jogador
- [ ] O `connect_token` entregue a cada jogador corresponde ao seu `playerID` (não troca tokens entre jogadores)
- [ ] Session responde `ResourceExhausted` -> todos os jogadores do match recebem `FAILED` (com `reason`) e o stream encerra
- [ ] `CreateSession` falha (session indisponível/erro) -> todos recebem `FAILED` e o stream encerra
- [ ] Após uma falha de provisionamento, o jogador consegue chamar `FindMatch` de novo e ser matched (não ficou preso)
- [ ] Inalterado: cancelar antes de formar -> sai da fila; sem token de auth -> `Unauthenticated`; um ticket por jogador

## Fora do escopo

- **Alterações no session-service** — ele já está pronto e estável; esta feature só o consome
- **Auth serviço-a-serviço** entre matchmaking e session (control plane interno no MVP, alinhado à spec do session) — evolução
- **Re-enfileiramento automático** em caso de falha de provisionamento (jogador re-tenta manualmente) — backlog
- **Retry/backoff** na chamada ao session e **circuit breaker** — backlog
- **Reportar o resultado** da partida (`ReportResult`) — é o game server quem chama o session diretamente; fora do matchmaking
- **Persistência/escala horizontal da fila** (Redis) — segue em memória, instância única (001)
