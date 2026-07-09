# ADR-0026: Alocação Agones fleet-aware por jogo (Fleet por game_id)

**Status:** Proposed
**Data:** 2026-07-09
**Serviço(s) afetado(s):** session-service, game-registry, admin-bff/admin-ui, data plane (Unity, indireto)
**Relaciona:** ADR-0011/0012 (Agones desde o início), ADR-0022 (ServerRegistry/reconciliação), ADR-0023 (perfil de multiplayer)

---

## Contexto

O adapter Agones do `session-service` (`internal/allocator/agones.go`) aloca sempre da **mesma Fleet fixa**: `main.go` lê `AGONES_FLEET` (default `"packet-arena"`) **uma vez no boot** e injeta em `NewAgones(client, namespace, fleet)`; toda `GameServerAllocation` usa esse `fleet` fixo no `matchLabels` do selector, para qualquer jogo.

A porta `domain.ServerAllocator.Allocate(ctx, matchID, gameID string)` **já recebe `gameID`** — mas hoje ele só vira annotation no `GameServer` alocado (`match_id`/`game_id`, sem prefixo), nunca entra na escolha da fleet. Ou seja, o "gancho" para alocação por jogo já existe na assinatura; só não está fiado.

Isso virou lacuna concreta, não hipotética: verificando a spec 002 do data plane `host_world_dropin` (2026-07-08/09), o `docker-compose.agones.yml` local desse repo aponta pro mesmo `AGONES_FLEET=packet-arena` do `queue_arena` — usar o override real do Agones ali alocaria um `GameServer` rodando o **binário errado** (queue_arena, não host_world_dropin). O único motivo de isso não ter causado uma sessão "presa" servindo o jogo errado é acidental: o `ServerBootstrap` do data plane valida o `archetype` no manifest (`GetSessionConfig`, ADR-0024) e se auto-mata em caso de mismatch — não é uma proteção do control plane. Na prática, a verificação da spec 002 teve que cair para o allocator `static` (compose base, sem override) + endereço fixo do server standalone, porque **não existe Fleet do Capoeira Souls no k3d ainda** e o allocator não saberia escolhê-la mesmo se existisse.

A pergunta que motivou este ADR (do usuário, sobre multi-jogo no mesmo cluster Agones): a segregação certa é por Fleet (1 por jogo/arquétipo, seletor lógico) ou por infraestrutura (nodes/namespaces por jogo)? E onde o `session-service` aprende qual Fleet usar?

## Alternativas Consideradas

### Opção A: Status quo — Fleet única fixa por env var
- **Prós:** zero trabalho; funciona enquanto só existir um jogo/arquétipo real no Agones.
- **Contras:** não escala para o 2º jogo; risco de alocar o **binário errado** silenciosamente (só não deu problema até agora por causa da validação de archetype no data plane, que é defesa do lado errado da borda); contradiz o que a própria assinatura de `Allocate` já sinalizava (`gameID` chega e é ignorado pra fleet).

### Opção B: Fleet por jogo, resolvida no `Allocate` via `game_id` (escolhida)
- **Prós:** fecha o gap que já estava meio-aberto na interface; cada jogo/arquétipo ganha sua própria Fleet/imagem sem risco de cross-binário; combina com o modelo já adotado (ADR-0023: perfil por jogo no game-registry) — o nome da fleet é só mais um campo do perfil.
- **Contras:** schema do `MultiplayerProfile` ganha um campo novo (versionamento); o adapter Agones deixa de fixar `fleet` na construção e passa a resolvê-lo por chamada (pequeno refactor em `agones.go`/`main.go`); operador precisa lembrar de criar a Fleet no k3d **antes** de habilitar Agones pra um jogo novo (item de runbook).

### Opção C: Fleet por (jogo × modo)
- **Contras:** o ADR-0024 já estabeleceu que modos do mesmo jogo compartilham o mesmo binário/Fleet (o server lê `params_json` no boot) — só arquétipo/jogo diferente precisa de imagem diferente. Multiplicar Fleet por modo é trabalho sem ganho. Descartada.

### Opção D: Segregação por infraestrutura (namespace/node por jogo)
- **Contras:** o scheduler do Kubernetes já distribui os pods sem problema; segregação física só se justifica por isolamento de capacidade (um jogo pesado que não pode competir por CPU com os outros) — problema separado, endereçável depois com taints/tolerations se e quando aparecer um caso real. Descartada como solução do problema atual.

## Decisão

Adotamos a **Opção B**. `MultiplayerProfile` (game-registry) ganha um campo **`Fleet string`** (nome da Fleet/label `agones.dev/fleet` a usar) — explícito, não uma convenção implícita `fleet = game_id`, porque o slug do jogo (UUID hoje) não é um bom nome de Fleet e porque jogos de teste podem querer compartilhar uma Fleet de dev.

`domain.ServerAllocator.Allocate` passa a receber o `fleet` resolvido (via o perfil, já buscado no fluxo de `OpenWorld`/`CreateSession` por ADR-0023 — sem chamada extra ao game-registry) em vez de o adapter Agones carregar um `fleet` fixo desde a construção. `NewAgones(client, namespace)` perde o parâmetro `fleet` do construtor; o selector da `GameServerAllocation` passa a montar `matchLabels: {agones.dev/fleet: fleet}` com o valor por-chamada. O adapter `static` (pool mock) ignora o campo, como já ignora `gameID` hoje.

Sem `Fleet` cadastrado no perfil → erro claro na criação da sessão (`FailedPrecondition`, mesma família do "sem perfil de multiplayer" que já existe) em vez de cair silenciosamente numa fleet default errada.

## Consequências

### Positivas
- Fecha um gap que já estava meio-fiado na própria assinatura da Port — não é uma feature nova do zero, é terminar o desenho existente.
- Múltiplos jogos/arquétipos no mesmo cluster Agones deixam de correr risco de alocação cross-binário.
- Consistente com ADR-0023: a Fleet vira só mais um atributo do perfil por jogo, mesma fonte canônica.

### Negativas / Trade-offs
- Schema do perfil (game-registry) muda — versionamento a considerar (ADR-0023 já apontava isso como pendência geral do perfil).
- Pequeno refactor em `session-service` (allocator + fluxo de resolução do perfil antes do `Allocate`).
- Processo operacional novo: toda Fleet precisa existir no cluster **antes** de o perfil apontar pra ela (nenhuma automação de criação de Fleet é proposta aqui — fica manual/Terraform, como hoje).

### Neutras / Implicações
- Não bloqueia nada até o **2º jogo real** precisar de alocação Agones de verdade — hoje só o queue_arena tem Fleet (`packet-arena`); o `host_world_dropin` (Unity, `unity-host-world-dropin-data-plane`) segue em modo `static` até sua própria Fase 5.
- Teste natural desta mudança: duas Fleets no k3d (`packet-arena` + a do Capoeira Souls) com sessões dos dois jogos alocando cada uma da fleet certa — vira o critério de aceite quando a Fase 5 daquele repo chegar.
- Runbook/doc operacional ganha o passo "criar a Fleet no k3d/Pi5 antes de cadastrar o perfil com Agones habilitado".

## Referências
- `services/session-service/internal/allocator/agones.go`, `internal/domain/session.go` (`ServerAllocator`), `cmd/server/main.go` (`AGONES_FLEET`) — estado atual investigado em 2026-07-09.
- ADR-0011/0012 — Agones como orquestrador.
- ADR-0022 — padrão de Port opcional (`ServerRegistry`) por adapter, mesmo espírito aplicável aqui.
- ADR-0023 — perfil de multiplayer por jogo (onde o campo `Fleet` encaixa).
- Achado empírico: `unity-host-world-dropin-data-plane`, verificação da spec 002 (2026-07-08/09) — override `docker-compose.agones.yml` hardcoded pro fleet do queue_arena.
