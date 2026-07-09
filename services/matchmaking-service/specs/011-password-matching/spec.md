# Spec: 011-password-matching

**Serviço:** matchmaking-service
**Status:** Done (2026-07-09)
**Data:** 2026-07-09
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, follow-up #1** (lado matchmaking: bypass por senha)
**Roadmap:** `docs/game-models/host_world_dropin-follow-ups.md` → item 1 (play-with-friends por senha)
**Depende de:** `matchmaking/006` (`Invade`/`Eligible`), `matchmaking/007` (`PostSummonSign`/`ListSigns`/`SummonSign`/`CoopEligible`), `session/020` (hash da senha no diretório de invadíveis)
**Emparelhada com:** `session/020-password-world-matching` e `unity-host-world-dropin-data-plane/docs/specs/003-invasion-signals-matchmaking` (Rodada 2 do plano conjunto)

## Contexto

`matchmaking/006` (invasão) e `matchmaking/007` (quadro de sinais) casam por **faixa** (`Eligible`/`CoopEligible`, bandas assimétrica/simétrica sobre soul level e weapon tier). O follow-up #1 (ficha 01 §4) pede um segundo caminho, **social**: se o invasor/aliado souber a **senha** do mundo do host, casa **ignorando** a faixa inteira — é como jogar com amigos combinados fora de banda, não descoberta por proximidade de nível.

A flag `PasswordEnabled` do perfil (`HostWorldDropInParams`, ADR-0023) já existe; esta spec é o primeiro consumidor dela. `session/020` (lado session, emparelhada) já expõe o que falta: `OpenWorld` aceita a senha do host (hasheada na sessão) e `ListInvadableWorlds` carrega o **hash** no diretório.

**Assimetria importante entre invasão e co-op** (ver `session/020` "Nota de design"): a invasão descobre o mundo por `ListInvadableWorlds` sem saber de antemão qual sessão é a certa — por isso precisa comparar contra o **hash vindo do session**. O co-op por sinais **não** passa pelo session-service nesta comparação: o aliado anuncia um sinal com uma senha (a que um amigo host lhe deu); o host, ao listar sinais do **seu próprio mundo**, informa a **mesma senha que ele mesmo definiu** — plaintext contra plaintext, inteiramente dentro do estado do matchmaking (o `Sign` já vive só aqui, spec 007). Não há round-trip novo ao session para o caminho de co-op.

## Comportamento

### Invasão por senha (`Invade`)

- WHEN o invasor chama `Invade(gameID, modeID, soulLevel, weaponTier, password)` com uma **senha não-vazia** → THE SYSTEM SHALL, para cada mundo do diretório (`ListInvadableWorlds`), comparar `HashWorldPassword(password)` contra o `password_hash` do mundo; **se bater** (ambos não-vazios e iguais) → THE SYSTEM SHALL considerar o mundo **elegível independentemente** do resultado de `Eligible` (faixa/weapon tier ignorados).
- WHEN a senha informada **não bate** com nenhum mundo (ou está vazia) → THE SYSTEM SHALL cair no caminho normal: `Eligible` decide por faixa, como hoje (`matchmaking/006`, sem regressão).
- THE SYSTEM SHALL manter a `InvasionEnabled`/`SupportsInvasion()` como o portão de entrada: senha **não** faz `Invade` funcionar num jogo/modo que não seja `host_world_dropin` com invasão habilitada — ela só isenta a faixa, não o arquétipo/flag geral.
- THE SYSTEM SHALL manter o comportamento de corrida (mundo enche entre listar e injetar → tenta o próximo elegível) igual, agora considerando "elegível" tanto por faixa quanto por senha.

### Co-op por senha (`PostSummonSign` + `ListSigns`)

- WHEN o aliado chama `PostSummonSign(gameID, modeID, soulLevel, weaponTier, password)` com senha não-vazia → THE SYSTEM SHALL guardar a senha (plaintext, em memória, junto do `Sign` — mesmo modelo de dados não-persistente da spec 007) além dos demais atributos do sinal.
- WHEN o host chama `ListSigns(gameID, modeID, hostSoulLevel, hostWeaponTier, password)` com a **mesma senha** que ele definiu no `OpenWorld` do seu mundo → THE SYSTEM SHALL considerar **elegível** (aparecer na lista) todo sinal cuja senha **bata** com a informada, **independentemente** de `CoopEligible` (faixa ignorada).
- WHEN não há senha em `ListSigns` (campo vazio) ou ela não bate com o sinal → THE SYSTEM SHALL cair no caminho normal: `CoopEligible` decide por faixa, como hoje (`matchmaking/007`, sem regressão). Sinal com senha mas host sem senha (ou senha errada) **não** aparece via bypass — pode ainda aparecer pelo caminho normal de faixa, se `CoopEligible` bater por conta própria.
- THE SYSTEM SHALL manter `CoopEnabled`/`SupportsCoop()` como portão de entrada, mesmo espírito da invasão.

### Compatibilidade

- `Eligible`/`CoopEligible` (specs 006/007) **permanecem inalteradas** — funções puras só de faixa. O bypass por senha é um **OR** aplicado no call site (`Invader.Invade`, `Summoner.ListSigns`), não uma mudança nas funções de elegibilidade.
- `FindMatch` (queue-form) — inalterado.

## Contratos de Interface

### Protobuf (`matchmaking/v1/matchmaking.proto`)

- `InvadeRequest` += `string password = 5` (índices 1–4 ocupados: `game_id`/`soul_level`/`weapon_tier`/`mode_id`).
- `PostSummonSignRequest` += `string password = 5` (idem, 1–4 ocupados).
- `ListSignsRequest` += `string password = 5` (idem, 1–4 ocupados).
- Regenerar com `buf` (nunca `sed`). Conferir índices livres reais no `.proto` atual antes de gerar.

### Ports (domínio)

- Nova função pura, **duplicada deliberadamente** de `session/020` (ver aquela spec, D2 — sem pacote crypto compartilhado entre serviços): `HashWorldPassword(plaintext string) string` (SHA-256 hex; vazio→vazio).
- `InvaderCriteria` (spec 006) ganha `Password string` (plaintext do invasor).
- `InvadableWorld` (espelho do session, spec 006/020) ganha `PasswordHash string`.
- `Sign` (spec 007) ganha `Password string` (plaintext do aliado, em memória).
- `CoopCriteria` ganha `Password string` (decisão de implementação, ajustada do desenho original: em vez de um parâmetro à parte, a senha entra direto na struct já reusada nos dois call sites — em `PostSign` representa a senha do aliado, em `ListSigns` a senha do host; comparada contra `Sign.Password`).
- **`WorldDirectory`** (spec 006, adapter `sessionclient`) passa a mapear também `password_hash` do `ListInvadableWorldsResponse` pro `InvadableWorld.PasswordHash`.

### Erros de domínio

- Nenhum novo — senha errada/vazia só faz cair no caminho de faixa existente (`ErrNoEligibleWorld`/lista sem o sinal), não um erro novo.

## Modelo de Dados

Sem estado persistente novo. `Sign.Password` vive no mesmo store em memória do sinal (spec 007) — mesma vida útil (TTL/claim), sem mudança de forma de armazenamento.

## Observabilidade

- Contador opcional `invasions_total{result="injected_by_password"}` (variante do `result="injected"` da spec 006) — separa quantas invasões vieram por senha vs. faixa, útil pro funil (H3). *(Decidir no fechamento se compensa a cardinalidade extra agora ou fica para H3, mesmo critério da spec 012/006.)*

## Fora de escopo

- **Hash compartilhado entre serviços** — deliberadamente duplicado (ver `session/020`).
- **Trocar/expirar a senha depois de posta** — fora, mesma postura da spec 007 para os demais atributos do sinal.
- **Senha com custo computacional (bcrypt/argon2)** — não é o modelo de ameaça (ver `session/020`, "Segurança / Observação de design").
- **UI/UX de "senha errada" com feedback rico** — o client (Unity, spec 003) recebe o mesmo `ErrNoEligibleWorld`/lista vazia de sempre; não há um erro dedicado "senha incorreta" (evita side-channel de enumeração, ainda que o modelo de ameaça seja baixo).

## Critérios de Aceite

- [x] `Invade` com senha que bate um mundo **fora** da faixa → injeta mesmo assim (bypass confirmado).
- [x] `Invade` com senha vazia ou que não bate nenhum mundo → comportamento idêntico ao `matchmaking/006` (regressão zero).
- [x] `Invade` num jogo/modo sem `InvasionEnabled` → `ErrInvasionNotSupported` mesmo com senha certa (portão de entrada não é isento).
- [x] `PostSummonSign` com senha + `ListSigns` do host com a mesma senha → sinal aparece **mesmo fora** da faixa simétrica de co-op.
- [x] `ListSigns` sem senha (ou senha errada) → comportamento idêntico ao `matchmaking/007` (regressão zero); sinal com senha ainda pode aparecer pelo caminho de faixa normal se `CoopEligible` bater sozinho.
- [x] `Eligible`/`CoopEligible` sem nenhuma mudança de assinatura ou comportamento interno — só o call site ganha o OR.
- [x] `FindMatch` (queue-form) sem regressão; testes com `-race`, ≥80% em `service/`/`domain/`.
