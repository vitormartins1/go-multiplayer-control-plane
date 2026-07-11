# Spec 022 — alocação Agones fleet-aware por perfil (ADR-0026)

**Serviço:** session-service
**Status:** Done
**Relaciona:** ADR-0026, ADR-0023 (perfil de multiplayer), spec 017 (mode snapshot + manifest)
**Depende de:** game-registry/012 (campo `fleet` no perfil). **Destrava:** 2º jogo real usando Agones sem risco de alocar o binário errado.

## Contexto

Hoje `session-service` aloca sempre da mesma Fleet Agones fixa, lida de `AGONES_FLEET` (default `"packet-arena"`) uma única vez no boot e injetada no adapter (`allocator.NewAgones(client, namespace, fleet)`). Isso funciona só enquanto existir um único jogo/arquétipo real usando Agones — o 2º jogo alocaria da mesma Fleet e receberia o **binário errado**, silenciosamente (só não aconteceu ainda porque o `ServerBootstrap` do data plane Unity se autovalida e se automata em caso de mismatch de arquétipo — defesa do lado errado da borda, não do control plane).

Esta spec fecha o gap: a Fleet passa a vir do jogo (não do perfil/modo — amendment do ADR-0026, ver game-registry/012: Fleet é por jogo porque todos os modos do mesmo jogo compartilham o mesmo binário, ADR-0024), snapshotada na sessão junto do resto do perfil (spec 017), resolvida por chamada em vez de fixa na construção do adapter. `registryclient.ModeProfile` já recebe o `fleet` do jogo embutido na resposta de `GET /multiplayer-profiles/{modeId}` (spec 012, item 5) — sem round-trip extra ao game-registry.

## Comportamento (EARS)

1. **QUANDO** `Create` (queue_arena/battle_royale/coop_mission) ou `OpenWorld` (host_world_dropin) busca o perfil do modo via `ProfileProvider` para montar o snapshot, **O SISTEMA DEVE** incluir o campo `Fleet` do jogo (vindo já embutido na resposta de `GET /multiplayer-profiles/{modeId}`, game-registry/012) no `ProfileSnapshot` resultante.
2. **QUANDO** o allocator ativo é o adapter Agones (não o `static`) **E** o `Fleet` do snapshot é uma string vazia, **O SISTEMA DEVE** recusar a criação da sessão/abertura de mundo com um erro da família "pré-condição não satisfeita" (mesmo padrão de `ErrProfileUnavailable` → `codes.FailedPrecondition`), sem chamar `Allocate`.
3. **QUANDO** o allocator ativo é o adapter Agones e o `Fleet` do snapshot é não-vazio, **O SISTEMA DEVE** chamar `Allocate` passando esse valor, e o `GameServerAllocation` criado **DEVE** usar `matchLabels: {"agones.dev/fleet": <valor>}` no selector — nunca um valor fixo do adapter.
4. **QUANDO** o allocator ativo é o adapter `static` (pool mockado), **O SISTEMA DEVE** ignorar o `Fleet` do snapshot (mesmo tratamento que já dá a `gameID` hoje) — vazio ou não, a sessão prossegue normalmente.
5. **O SISTEMA DEVE** remover a env var `AGONES_FLEET` e o parâmetro `fleet` do construtor `NewAgones` — não deve sobrar nenhum caminho de código que aloque de uma Fleet fixa hardcoded/configurada globalmente.
6. **QUANDO** o manifest do data plane (`GetSessionConfig`, spec 017) é montado, **O SISTEMA PODE** (não é obrigatório nesta spec — só se trivial) incluir o `Fleet` por completude informativa; não é consumido pelo Unity (quem lê Fleet é só o Agones, via label do GameServer).

## Critérios de aceite

- Teste de unidade do `Agones` adapter: `Allocate(ctx, matchID, gameID, fleet)` monta o selector com o `fleet` recebido por chamada (não um campo fixo da struct); duas chamadas com fleets diferentes produzem selectors diferentes.
- Teste de serviço: `Create`/`OpenWorld` com snapshot `Fleet: ""` e allocator Agones → erro `FailedPrecondition`, `Allocate` **não** é chamado (mock `AssertNotCalled`).
- Teste de serviço: mesmo cenário com allocator `static` → sessão criada normalmente, sem erro.
- Teste de serviço: snapshot `Fleet: "packet-arena"` e allocator Agones → `Allocate` chamado com `fleet="packet-arena"`.
- `main.go` não lê mais `AGONES_FLEET`; `NewAgones` tem assinatura `(client, namespace)`.
- Suite completa do serviço (`go test ./... -race -cover`) verde; cobertura ≥ 80% mantida em `service/`/`domain/`.

## Rollout / operacional (não é código, mas é pré-requisito de deploy)

- Antes de habilitar esta spec em qualquer ambiente com Agones real ativo (hoje: só o `queue_arena`, Fleet `packet-arena`), o perfil do jogo correspondente precisa ter `fleet: "packet-arena"` setado via game-registry (spec 012) — senão a primeira sessão depois do deploy falha com `FailedPrecondition`. É um passo manual único, documentado aqui e no runbook (não há migração automática: `fleet` é atributo por perfil/jogo, não um default de schema).
- `host_world_dropin` (`unity-host-world-dropin-data-plane`) segue no adapter `static` até sua própria Fase 5 — item 4 acima garante que nada muda para ele nesta spec.

**Executado (2026-07-10):**
- `PUT /games/{id}/fleet` chamado para os dois jogos de referência: **Packet Arena** (`e4c9c5b4-e54f-4884-b377-ace16160cf8b`) → `fleet: "packet-arena"`; **Capoeira Souls** (`274a9171-83c7-407b-bc43-e4a7730134c8`, `host_world_dropin`) → `fleet: "capoeira-souls"` (pré-cadastrado mesmo sem Fleet Kubernetes própria ainda — a Fase 5 daquele repo só precisa criar o `fleet.yaml`, o lado do control plane já está pronto).
- `infra/docker/docker-compose.agones.yml` e `docker-compose.agones-pi5.yml` tinham `AGONES_FLEET: packet-arena` sobrando (env var morta desde a remoção do item 5) — removido dos dois.
- Docs de referência cross-repo atualizadas nos dois repos Unity (`unity-queue-arena-data-plane/docs/ROADMAP.md`, `unity-host-world-dropin-data-plane/docs/ROADMAP.md` + `CLAUDE.md`) — cada um com uma nota "o que mudou no backend e o que fazer aqui" para quem abrir sessão de trabalho naquelas pastas.

## Fora de escopo

- Criar a Fleet no cluster (k3d/Pi5) — trabalho de infra (Terraform/manifests Agones), item de runbook separado, não desta spec.
- Fleet por (jogo × modo) — descartado no ADR-0026 (Opção C): modos do mesmo jogo compartilham binário/Fleet.
- Qualquer UI de console para o campo `Fleet` — fora de escopo também na spec 012 (game-registry); pode ser operado via API enquanto isso.
