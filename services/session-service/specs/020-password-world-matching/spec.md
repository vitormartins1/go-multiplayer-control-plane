# Spec: 020-password-world-matching

**Serviço:** session-service
**Status:** Done (2026-07-09)
**Data:** 2026-07-09
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, follow-up #1** (lado session: senha do mundo)
**Roadmap:** `docs/game-models/host_world_dropin-follow-ups.md` → item 1 (play-with-friends por senha)
**Depende de:** `session/011` (`OpenWorld`), `session/012` (`ListInvadableWorlds`, atributos do host)
**Emparelhada com:** `matchmaking/011-password-matching` (lado matchmaking, consome o que esta spec expõe) e `unity-host-world-dropin-data-plane/docs/specs/003-invasion-signals-matchmaking` (Rodada 2 do plano conjunto)

## Contexto

A flag `PasswordEnabled` já existe no perfil `host_world_dropin` (`HostWorldDropInParams`, ADR-0023) e o Capoeira Souls já tem o modo `batizado` cadastrado com ela ligada (`unity-host-world-dropin-data-plane/docs/MODES.md`) — mas **nenhum serviço a consome**. Hoje `OpenWorld` não aceita senha nenhuma; `ListInvadableWorlds` (spec 012) não carrega nada que permita casar por senha.

O caso de uso (ficha 01 §4, "bateu a senha, ignora a faixa"): o host abre o mundo com uma senha combinada fora de banda (chat, Discord); quem souber a senha entra **ignorando** faixa de nível/weapon tier — é o caminho social de jogar com amigos, sem depender do range assimétrico da invasão (`matchmaking/006`) nem da banda simétrica do co-op (`matchmaking/007`).

Esta spec cobre o **lado session**: o `OpenWorld` passa a aceitar uma senha opcional do host, guardada **hasheada** (nunca em texto puro na sessão em memória — evita expor a senha se o admin console algum dia listar detalhes de sessão). O diretório de mundos invadíveis (`ListInvadableWorlds`) passa a carregar esse hash, para o matchmaking (`matchmaking/011`) comparar contra a senha que o invasor/aliado forneceu. A **política de bypass** (quando a senha bate, ignorar `Eligible`/`CoopEligible`) mora inteiramente no matchmaking — aqui o session continua **fonte de dados pura**, mesmo princípio da spec 012.

> **Nota de design:** o co-op por sinais (`matchmaking/007` — `PostSummonSign`/`ListSigns`/`SummonSign`) **não** precisa que o session exponha nada novo: a comparação de senha ali acontece inteiramente dentro do matchmaking (o aliado anuncia com uma senha; o host, ao listar, informa a mesma senha que ele mesmo definiu no `OpenWorld` — plaintext-contra-plaintext, sem round-trip ao session). Só a **invasão** precisa do hash no diretório, porque o invasor descobre o mundo pelo `ListInvadableWorlds` sem saber de antemão qual sessão é a certa.

## Comportamento

### Senha no `OpenWorld`

- WHEN o host chama `OpenWorld` informando uma **senha não-vazia** → THE SYSTEM SHALL hashear a senha (SHA-256, hex) e guardar o hash na sessão. A senha em texto puro **nunca** é persistida nem logada.
- WHEN o host chama `OpenWorld` **sem** senha (campo vazio) → THE SYSTEM SHALL abrir o mundo normalmente, sem hash guardado (nulo/vazio) — nenhuma regressão sobre a H0/H1a.
- THE SYSTEM SHALL manter todo o resto do `OpenWorld` inalterado (atributos de matchmaking, slots, token do host).

### Hash no diretório de invadíveis

- WHEN o matchmaking chama `ListInvadableWorlds(gameID)` → THE SYSTEM SHALL incluir, para cada mundo, o **hash da senha** (vazio se o host não definiu senha) — o mesmo campo que `matchmaking/011` compara contra a senha fornecida pelo invasor.
- THE SYSTEM SHALL **não** aplicar nenhuma lógica de bypass aqui — comparar hash e decidir elegibilidade é política do matchmaking (mesma separação da spec 012 para faixa/weapon tier).

### Compatibilidade

- `queue_arena` e mundos `host_world` sem senha seguem sem mudança observável. O campo é **aditivo** em `OpenWorldRequest` e em `InvadableWorld`.

## Contratos de Interface

### Protobuf (`session/v1/session.proto`)

- `OpenWorldRequest` ganha campo **opcional**: `string password = 6` (próximo índice livre; ausente/vazio = sem senha).
- `InvadableWorld` ganha `string password_hash = 5` (vazio = mundo sem senha).
- Regenerar com `buf` (nunca `sed`).

### Ports (domínio)

- **`WorldAttributes`** (spec 012) ganha `Password string` (plaintext recebido, só usado para computar o hash no momento do `OpenWorld` — não sobrevive além disso).
- **`Session`** ganha `PasswordHash string` (vazio = sem senha).
- **`InvadableWorld`** ganha `PasswordHash string`.
- Nova função pura em `domain`: `HashWorldPassword(plaintext string) string` (SHA-256 hex; string vazia → hash vazio, não o hash de `""`). Usada só aqui — `matchmaking/011` implementa a **mesma** função de forma independente (decisão deliberada, ver plan D-hash) para não acoplar os dois serviços a um pacote crypto compartilhado.

### Erros de domínio

- Nenhum novo.

## Modelo de Dados

Sessão continua em memória (H0/H1a). `PasswordHash` entra na mesma struct `Session`. **Sem migration.**

## Segurança / Observação de design

- Este "password" é um **código de convite social** (baixo risco, mesma categoria de um link de sala), não uma credencial de autenticação — o hash SHA-256 simples (sem sal/custo, ao contrário de bcrypt) é proporcional: o objetivo é não deixar a senha em texto puro visível no estado da sessão ou num futuro dump do admin console, **não** resistir a ataque de força bruta direcionado. Documentar essa fronteira explicitamente onde o código mexe, para não confundir com autenticação de verdade.

## Fora de escopo

- **Bypass de elegibilidade** (a lógica de "senha bate → ignora faixa") — `matchmaking/011`.
- **Senha no co-op** (`PostSummonSign`/`ListSigns`) — não toca o session (ver Nota de design acima).
- **Alterar/remover a senha após `OpenWorld`** — não suportado nesta fatia; senha é fixada na abertura do mundo.
- **Rotação/expiração de senha** — não modelado.

## Critérios de Aceite

- [x] `OpenWorld` com senha guarda o hash na sessão; sem senha, abre igual (hash vazio).
- [x] `ListInvadableWorlds` inclui `password_hash` de cada mundo (vazio quando não há senha).
- [x] `HashWorldPassword("")` não é igual a `HashWorldPassword` de nenhuma senha real (mundo sem senha nunca "bate" por acidente com um invasor que não informou senha).
- [x] `queue_arena` e H0/H1a sem regressão; testes com `-race` (155 verdes), 94.1%/100% em `service/`/`domain/`.
