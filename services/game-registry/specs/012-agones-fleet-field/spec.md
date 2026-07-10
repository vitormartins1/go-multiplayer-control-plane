# Spec 012 — campo `Fleet` por jogo (ADR-0026)

**Serviço:** game-registry
**Relaciona:** ADR-0026 (alocação Agones fleet-aware por jogo), ADR-0023 (perfil de multiplayer), ADR-0024/spec 009 (modos nomeados)
**Depende de:** nada. **Destrava:** session-service/022 (fleet-aware allocation).

## Contexto

O ADR-0026 decidiu que a alocação Agones deixa de usar uma Fleet fixa (`AGONES_FLEET`, env var do session-service) e passa a resolver a Fleet **por jogo**. O texto original do ADR propôs o campo em `MultiplayerProfile` — mas isso contradiz o próprio argumento que descartou a "Opção C" (Fleet por jogo×modo) no ADR: modos do mesmo jogo compartilham o **mesmo binário** (o server lê `params_json` no boot para se configurar por modo, ADR-0024), então só faz sentido uma Fleet por **jogo**, não por modo. Amendment confirmado com o usuário em 2026-07-10: o campo `Fleet` vive em `domain.Game` (game-registry, `internal/domain/game.go:20-36`), irmão de `MultiplayerModes`, não dentro de cada `MultiplayerProfile`.

## Comportamento (EARS)

1. **O SISTEMA DEVE** expor um endpoint dedicado `PUT /games/{id}/fleet` que recebe `{"fleet": "<slug>"}` e persiste o valor no jogo (não no perfil de nenhum modo).
2. **QUANDO** o valor de `fleet` é não-vazio, **O SISTEMA DEVE** validar contra `^[a-z][a-z0-9-]{0,31}$` (mesmo formato de slug já usado para `modeId` — compatível com o label `agones.dev/fleet` do Kubernetes) antes de persistir; valor fora do formato → `400 Bad Request`.
3. **QUANDO** o valor de `fleet` é string vazia, **O SISTEMA DEVE** aceitar (limpa o campo — jogo sem Fleet configurada).
4. **O SISTEMA DEVE** devolver o `fleet` do jogo em `GET /games/{id}` (junto dos demais campos do jogo, não dentro de `multiplayerProfile`/`multiplayerModes`).
5. **QUANDO** o session-service busca um modo via `GET /games/{gameId}/multiplayer-profiles/{modeId}` (endpoint já consumido por `registryclient.ModeProfile`, spec 017), **O SISTEMA DEVE** incluir o `fleet` do jogo dono do modo na resposta — mesma leitura em uma única chamada, sem round-trip extra ao game-registry (essa é a exigência que o ADR-0026 já fazia: "sem chamada extra ao game-registry").
6. **QUANDO** um jogo pré-existente (criado antes desta spec) é lido, **O SISTEMA DEVE** devolver `fleet: ""` (zero value), nunca omitir o campo nem quebrar a decodificação.

## Critérios de aceite

- `PUT /games/{id}/fleet` com `{"fleet":"packet-arena"}` → `200`; `GET /games/{id}` subsequente devolve `"fleet":"packet-arena"`.
- `PUT /games/{id}/fleet` com `{"fleet":"Packet_Arena"}` (maiúscula/underscore) → `400`.
- `PUT /games/{id}/fleet` com `{"fleet":""}` → `200`, limpa o valor.
- `GET /games/{gameId}/multiplayer-profiles/{modeId}` de um jogo com `fleet` setado → resposta inclui `"fleet":"<valor>"` junto do modo.
- Jogo pré-existente (seed/fixture anterior a esta spec) → `GET /games/{id}` devolve `"fleet":""`, sem erro de decodificação.
- Cobertura ≥ 80% em `service/`/`domain/` (padrão do projeto) para os caminhos novos.

## Fora de escopo

- Qualquer lógica de "qual Fleet este jogo deveria usar" — decisão operacional/manual do usuário do console.
- Enforcement de obrigatoriedade (fleet vazio + allocator Agones = erro) — vive no session-service (spec 022), que é quem sabe qual allocator está ativo.
- UI do console (admin-ui) para editar o campo — pode ser operado via API enquanto a UI não existe; não bloqueia a spec 022.
- Fleet por modo — descartado (ver Contexto); se um caso real exigir binários diferentes por modo do mesmo jogo, é uma revisão de ADR, não uma extensão aditiva desta spec.
