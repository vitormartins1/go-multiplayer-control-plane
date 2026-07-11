# Plan: 022-mode-snapshot-icon
**Spec:** specs/022-mode-snapshot-icon/spec.md
**Data:** 2026-07-11

## Abordagem técnica

O ponto central é **não duplicar** o cálculo de hash: extrair a função hoje presa em `session-service/internal/registryclient/client.go` (`profileHash`) pra `shared/modesnapshot`, e fazer os DOIS lados (session-service real, admin-bff "e se eu abrisse uma sessão agora") chamarem a mesma função. Isso é o que garante a promessa da spec ("o ícone de Configurações já mostra o que a próxima sessão vai carregar") sem risco de os dois cálculos divergirem com o tempo.

O admin-bff já tem tudo que precisa pra calcular o hash sem round-trip extra ao game-registry: `ListMultiplayerModes(gameID)` já devolve o array cru (`json.RawMessage`) com `{modeId, archetype, topology, params}` por item — mesma forma que o session-service decodifica pra tirar o hash de UM modo. Só precisa decodificar esse array (preservando `params` como `json.RawMessage` no struct de decode, pra não perder a fidelidade de bytes) e aplicar `modesnapshot.Hash` em cada item.

No admin-ui, a mudança de seed (item 1-3 da spec) é local/trivial (`SessionAvatar seed={...}`). A parte de layout (Rooms, item 12-14) é a mais delicada: hoje não existe `GameCover` nenhum nas linhas de Rooms — precisa ser adicionado, e o overlay do ícone de partida usa `position: absolute` sobre um `Box` com `position: relative` (mesmo padrão que o próprio `GameCover` já usa internamente pra sobrepor o ícone de arquétipo no canto superior-esquerdo — replicar a técnica, canto oposto).

## Arquivos a criar ou modificar

**shared/ (novo pacote):**
- `shared/modesnapshot/hash.go` — `func Hash(archetype, topology string, params json.RawMessage) string` (corpo idêntico ao `profileHash` atual do session-service)
- `shared/modesnapshot/hash_test.go` — casos determinísticos (mesma entrada → mesmo hash; entradas diferentes → hashes diferentes; `params` vazio não quebra)

**session-service:**
- `internal/registryclient/client.go` — remove a função local `profileHash`; `ModeProfile` passa a chamar `modesnapshot.Hash(m.Archetype, m.Topology, m.Params)`
- Testes existentes de `registryclient` continuam valendo sem alteração (mesmo hash, fonte diferente)

**admin-bff:**
- `internal/domain/admin.go` (ou arquivo de fleet/modo dedicado) — método novo na Port `GameRegistry`: `ModeSnapshotHashes(ctx, gameID string) (map[string]string, error)`
- `internal/gameregistry/client.go` — implementa `ModeSnapshotHashes`: chama `ListMultiplayerModes` (já existe), decodifica em `[]struct{ModeID, Archetype, Topology string; Params json.RawMessage}`, aplica `modesnapshot.Hash` por item, monta o mapa
- `internal/service/admin_service.go` — método `Service.ModeSnapshotHashes(ctx, gameID)` delegando ao registry
- `internal/transport/http_handler.go` — rota `GET /api/games/{gameId}/multiplayer-profiles/snapshot-hashes` → handler `modeSnapshotHashes`

**admin-ui:**
- `src/lib/api.ts` — `api.modeSnapshotHashes(gameId, signal)`
- `src/pages/GameSessions.tsx` — `SessionAvatar seed={s.profileHash || s.sessionId}` (2 call sites); badge de modo troca `color="indigo"` por ícone de `ARCHETYPE_META[s.archetype]` + `color="gray"`
- `src/pages/Rooms.tsx` — idem seed (2 call sites) + idem badge de modo; primeira coluna nova (`GameCover` variante pequena + `SessionAvatar` sobreposto); coluna "Match" perde o `SessionAvatar`
- `src/pages/MultiplayerSettings.tsx` — busca `api.modeSnapshotHashes` (uma vez, junto do carregamento da tabela); coluna nova de ícone (primeira) + coluna nova de hash (texto monoespaçado)

## Compute e banco (ADR-0005)

N/A — nenhuma mudança de compute/banco; tudo client-side (hash) ou passthrough de dados já buscados.

## Dependências e libs

Nenhuma nova — `crypto/sha256`/`encoding/hex`/`encoding/json` já são stdlib; DiceBear já é dependência do admin-ui.

## Decisões técnicas e trade-offs

- **Hash compartilhado via `shared/`, não duplicado nem exposto por HTTP entre os dois serviços Go.** Alternativa considerada: admin-bff replicar a lógica localmente (copiar o código). Rejeitada: risco de drift silencioso se alguém mudar um lado e esquecer o outro — exatamente o tipo de bug que some do "confere ao vivo" e só aparece quando alguém compara hashes manualmente. `shared/` já é o padrão do monorepo pra isso (ADR-0001, módulo único).
- **Endpoint novo devolve hashes de TODOS os modos do jogo numa chamada** (mapa `modeId → hash`), não um endpoint por modo. Alternativa considerada: `GET .../multiplayer-profiles/{modeId}/snapshot-hash` (um por modo). Rejeitada: `MultiplayerSettings.tsx` já lista todos os modos de uma vez — um endpoint por modo forçaria N chamadas do frontend só pra popular uma tabela que já carrega tudo de uma vez hoje.
- **Overlay do ícone de partida no canto inferior-direito da capa** (Rooms, item 12) — o canto superior-esquerdo já é ocupado pelo ícone de arquétipo dentro do próprio `GameCover`. Decisão de layout, não crítica; se ficar visualmente ruim na prática (ex. capa pequena demais pros dois badges), revisar durante a implementação — não é um critério de aceite duro da spec, é o ponto de partida.
- **Não recalcula/cacheia hash no admin-bff** — mesma postura de baixo-tráfego já usada na spec 023 (fleet status).

## Ordem de implementação

1. `shared/modesnapshot` (com teste) → session-service adota (sem mudar comportamento, só a origem do código) → `go test ./services/session-service/...` verde
2. admin-bff: Port → client → service → transport (com testes em cada camada) → `go test ./services/admin-bff/...` verde
3. admin-ui: seed do `SessionAvatar` (troca simples, 4 call sites) — menor risco, fazer primeiro pra validar visualmente o "ícone muda com o modo"
4. admin-ui: badge de modo (ícone de arquétipo) em Rooms/GameSessions
5. admin-ui: `MultiplayerSettings.tsx` (colunas nova de ícone + hash)
6. admin-ui: `Rooms.tsx` primeira coluna (capa + overlay) — a parte mais trabalhosa de CSS/layout, por último
7. Verificação end-to-end: mudar um modo em Configurações, confirmar ícone mudou ali E que abrir uma sessão nova daquele modo gera o MESMO ícone em Rooms/Sessions

## Riscos e pontos de atenção

- **Fidelidade de bytes do `json.RawMessage`:** o hash depende de `params` chegar intocado (mesma ordem de chaves/espaçamento) da resposta HTTP até a função de hash — confirmar que `ListMultiplayerModes` (admin-bff) decodifica com um struct que tem `Params json.RawMessage` (não `map[string]interface{}`, que perderia a formatação original) antes de aplicar `modesnapshot.Hash`. Testado no caso de aceite "hash do endpoint == profileHash de uma sessão real aberta com aquela config".
- **`GameCover` variante pequena em `Rooms.tsx`:** confirmar que a variante "mini" já usada no seletor do header tem o tamanho/proporção certos pra uma coluna de tabela (provavelmente precisa de um `size`/prop novo, não necessariamente reaproveitar 1:1) — decidir durante a implementação, não é uma decisão de arquitetura, é ajuste visual.
- **Sessões sem `archetype`** (legadas, spec anterior à 014/018) — já tratado no EARS item 16, mas vale conferir com um fixture de teste real (sessão antiga do ambiente de dev, se existir) em vez de só um mock.
