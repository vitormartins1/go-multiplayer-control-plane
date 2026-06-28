# Mapa de Features — game-backend

> Planejamento de **alto nível** das features de cada serviço. Guia estável para enxergar o panorama, prioridades e dependências.
> As specs detalhadas (`spec→plan→tasks`) são escritas **just-in-time**, por serviço, conforme chegamos nele — informadas pelo que aprendermos implementando os anteriores.
>
> **Prioridades:** `[MVP]` necessário para o loop ponta a ponta de teste · `[próximo]` logo depois · `[backlog]` futuro/se necessário.
> Cada linha vira uma feature SDD (pasta `NNN-slug`), agrupada por coesão de entrega (como no identity).

---

## Loop MVP (o que precisa existir para testar o jogo ponta a ponta)

login/anônimo (identity) → find match (matchmaking) → aloca server + connect token (session) → partida (Unity) → resultado → atualiza leaderboard/achievements (+ persistência player-data/cloud-save). Analytics tem prioridade menor.

---

## 1. identity-service (≈ GameKit Identity & Auth) — **planejado**

*Fundação de auth dos demais. Não depende de ninguém.*

- `001-auth-core` — contas + login + session JWT + chaves Ed25519 + JWKS — **✅ Approved**
- `002-anonymous-login` — login guest stateless (claim `anon`) — **✅ Approved**
- Login federado (Steam/Google) — `[backlog]`
- Account linking (guest → conta) — `[backlog]`

## 2. achievements-service (≈ GameKit Achievements)

*Depende: valida JWT da identity; recebe eventos de gameplay/resultado.*

- Catálogo de conquistas (id, nome, descrição, critério) — `[MVP]`
- Desbloqueio por evento de gameplay (evento → avalia regra → desbloqueia) — `[MVP]`
- Idempotência de desbloqueio (não desbloquear 2x) — `[MVP]`
- Listar conquistas do jogador (desbloqueadas + progresso) — `[MVP]`
- Progresso parcial (contadores, ex: 7/10) — `[próximo]`
- Conquistas secretas / recompensas associadas — `[backlog]`

## 3. player-data-service (≈ GameKit User Gameplay Data)

*Depende: valida JWT da identity. Dados estruturados e pequenos (inventário, stats), por jogador.*

- Ler/gravar dados por jogador em chave/namespace (inventory, stats, settings) — `[MVP]`
- Persistência server-side = cross-play automático — `[MVP]`
- Controle de concorrência (optimistic locking, evitar sobrescrita) — `[próximo]`
- Limites/quota de tamanho por jogador — `[backlog]`
- Operações em lote (bulk get/set) — `[backlog]`

## 4. cloud-save-service (≈ GameKit Game State Saving)

*Depende: valida JWT da identity. Blob opaco maior (arquivo de save), distinto do player-data.*

- Salvar slot de save (blob do estado do jogo) — `[MVP]`
- Carregar save — `[MVP]`
- Versionamento de save (número/histórico de versão) — `[próximo]`
- Resolução de conflito (cliente vs nuvem) — `[próximo]`
- Múltiplos slots por jogador — `[próximo]`
- Compressão / sync delta — `[backlog]`

## 5. leaderboard-service (≈ ref. arch DynamoDB/Redis)

*Depende: valida JWT (submissão); recebe resultado da partida (session).*

- Submeter score/resultado — `[MVP]`
- Ranking global (top N) — `[MVP]`
- Ranking ao redor do jogador (rank ±k) — `[MVP]`
- Janelas temporais (diário/semanal/all-time, TTL) — `[próximo]`
- Múltiplos leaderboards (por modo/temporada) — `[próximo]`
- Validação/anti-cheat de score — `[backlog]`

## 6. analytics-service (≈ Game Analytics Pipeline) — *prioridade menor*

*Depende: recebe eventos de telemetria.*

- Ingestão de eventos de telemetria — `[MVP da fase]`
- Worker pool processando eventos (goroutines + channels) — `[MVP da fase]` *(o ponto didático de concorrência)*
- Persistência/agregação (contadores, métricas) — `[MVP da fase]`
- Consumo via SQS (AWS SDK Go v2) — `[próximo]`
- Dashboards de eventos no Grafana — `[próximo]`
- Sessionization / funis / retenção — `[backlog]`

## 7. matchmaking-service (≈ GameLift FlexMatch) — *gRPC*

*Depende: valida JWT; chama o session-service ao formar a partida.*

- Entrar na fila / criar ticket — `[MVP]`
- Status do ticket via gRPC streaming (em fila / match / pronto) — `[MVP]`
- Cancelar ticket / sair da fila — `[MVP]`
- Formação de match por regras (skill, tamanho de time) — `[MVP]`
- Ao formar match → chama session-service para alocar server — `[MVP]`
- Agrupamento por latência/região — `[próximo]`
- Relaxamento progressivo das regras (expandir após tempo) — `[próximo]`
- Party/grupos, backfill — `[backlog]`

## 8. session-service (≈ GameLift — orquestração de frota/sessão)

*Depende: matchmaking (recebe match); identity (chave do connect token); Agones (alocação).*

- Criar sessão a partir de um match — `[MVP]`
- Alocar game server (cliente da Allocation API do Agones) — `[MVP]`
- Emitir connect token (Ed25519, ver ARCHITECTURE §7) — `[MVP]`
- Receber resultado da partida (callback do game server) — `[MVP]`
- Fan-out do resultado → leaderboard / achievements / analytics — `[MVP]`
- Registry/heartbeat próprio (caso sem Agones, nível menor) — `[próximo]`
- Reconnection / spectator / migração de sessão — `[backlog]`

---

## Concerns compartilhados (cross-service) — decidir na hora certa

- **Middleware de auth (`shared/middleware`):** valida o JWT da identity via JWKS. Usado por todos os serviços autenticados. Nasce quando o segundo serviço precisar autenticar.
- **Chave do connect token:** a identity é a fundação de auth e detém as chaves de assinatura. Definir (no plan do session) se o session-service assina com a chave do identity (via chamada/segredo compartilhado) ou tem chave própria publicada no mesmo JWKS.
- **Fluxo de resultado/eventos:** o resultado da partida faz fan-out para leaderboard/achievements/analytics. Decidir (quando chegar no session) se é **chamada direta** (síncrona) ou **event-driven** (SQS/eventos). Provável ADR.
- **Contratos `.proto`:** matchmaking/session expõem gRPC; os `.proto` são compartilhados com o repo Unity como contrato.
