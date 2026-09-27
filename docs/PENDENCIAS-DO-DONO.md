# Pendências do dono — o que depende de você, e de onde retomar

> Tudo que o código já não resolve sozinho: burocracia (Steam), decisões de produto, trabalho no projeto Unity e
> validações que a VM da revisão não conseguiu fazer. Cada item diz **o que fazer**, **onde** e **como saber que
> terminou**. Estado em 2026-09-27, branch `claude/revisao-produto-l2k7hr`.
>
> Histórico do que foi feito: [`review/PROGRESSO.md`](review/PROGRESSO.md). Estado geral: [`NEXT-STEPS.md`](NEXT-STEPS.md).

## 0. Primeiro de tudo: trazer a branch para a `main`

Toda a revisão (2026-09-26 e 27) está na branch `claude/revisao-produto-l2k7hr`, sem PR aberto. O caminho de sempre
(ADR-0013): revisar e fazer `merge --no-ff` na `main`, ou pedir para abrir o PR. O CI roda na primeira vez nesse
push — veja o item 5 (govulncheck).

## 1. Steam — chaves e cadastro (burocracia)

O backend já faz **login Steam** (identity) e **espelha conquistas** na Steam (achievements); os dois estão
desligados até receberem as chaves. Foram validados contra uma Steam falsa.

**1.1 Pegar as credenciais no Steamworks** (painel do parceiro; os nomes dos menus podem ter mudado — confira):
- **App ID** do Folclore Souls (página do app).
- **Publisher Web API Key**: Users & Permissions → Manage Groups → o grupo do app → criar a WebAPI Key. É um
  segredo: não vai para o git, nem para o cliente do jogo.

**1.2 Cadastrar as conquistas no Steamworks** (Stats & Achievements), cada uma com um **API Name** (ex.:
`ACH_BOSS_BOITATA`), e **publicar** as mudanças do Steamworks.

**1.3 Configurar o backend** (variáveis de ambiente; no compose elas vêm do shell ou de um `.env` ao lado do
`infra/docker/docker-compose.yml`):

| Serviço | Variáveis |
|---|---|
| `identity-service` | `STEAM_WEB_API_KEY`, `STEAM_APP_ID`, `STEAM_TICKET_IDENTITY` (padrão `control-plane`; o jogo pede o ticket com o mesmo texto) |
| `achievements-consumer` | `STEAM_WEB_API_KEY`, `STEAM_APP_ID`, `STEAM_GAME_ID` (o **id do jogo no control plane**, visto na URL do console) |

**1.4 No console:** em cada conquista do Folclore, preencher **Nome na Steam** com o API Name do 1.2 (aparece o selo
"Steam" no catálogo). Quem já tinha desbloqueado recebe na Steam também (backfill automático).

**Pronto quando:** login com um ticket real devolve JWT (`POST /sessions/steam`); ao desbloquear uma conquista, a
métrica `achievements_steam_sync_total{result="synced"}` sobe e a conquista aparece no perfil Steam da conta de
teste. Se aparecer `retry`/`failed`, o motivo está em `player_achievements.steam_sync_error`.

**Limite conhecido:** um app Steam por deploy (um `STEAM_APP_ID` ↔ um `STEAM_GAME_ID`). Outro jogo na Steam pede
evoluir isso (ver seção 6).

## 2. Projeto Unity — o trabalho do lado do jogo

Tudo documentado em [`integration/UNITY-GUIA-INTEGRACAO.md`](integration/UNITY-GUIA-INTEGRACAO.md), com um **prompt
pronto** para o agente do projeto Unity. Ordem:

1. Credenciais no gRPC do session (bearer no cliente, `x-game-server-key` no DS) — contrato 1.2.0.
2. Login Steam no cliente — 1.3.0.
3. `mode_id` igual no `OpenWorld` e no `Invade`; boss em `stats` — 1.1.0.
4. Heartbeat no DS (sem ele o mundo fecha aos 30 min) — 1.5.0.
5. P2P: decidir o transporte Steam do Netcode for GameObjects (o guia pede ao agente que proponha e espere sua
   aprovação), implementar host e phantom — 1.4.0.
6. Não chamar `SetAchievement` no cliente para conquistas espelhadas pelo backend.

**Pronto quando:** o checklist da seção 6 do guia estiver todo marcado.

## 3. Ligar o `enforce` da autenticação do session

Hoje o session aceita chamada sem credencial (modo `warn`, só conta). Depois do passo 2.1:

1. Rodar uma sessão completa do jogo e conferir `session_auth_rejections_total` = 0 (Grafana ou `/metrics` do
   session).
2. Trocar `SESSION_AUTH_MODE` para `enforce` no compose (`infra/docker/docker-compose.yml`, serviço
   `session-service`) e onde mais o session rodar.
3. Registrar a versão **2.0.0** em `docs/contracts/CHANGELOG.md` (é quebra: cliente antigo sem credencial para de
   funcionar).

## 4. Virar o Folclore para P2P

Depois do passo 2.5: no console, Configurações do jogo → modo → **Topologia = P2P**. Em Rooms, os mundos passam a
mostrar "P2P · steam:…" e o sinal de vida. O backend está pronto desde a ADR-0032.

## 5. CI — `govulncheck`

O job `govulncheck` entrou como **report-only** (`continue-on-error: true` em `.github/workflows/ci.yml`) porque a VM
não alcança o `vuln.go.dev`. No primeiro run do CI: ler o resultado; atualizar as dependências apontadas (ou abrir a
tarefa); depois remover o `continue-on-error` para o job bloquear.

## 6. Decisões de produto em aberto

| Decisão | Recomendação | Onde está o contexto |
|---|---|---|
| Consolidar as 5 tabelas `*_daily` do analytics numa só | sim, quando mexer de novo no analytics — pede uma ADR nova sobre a ADR-0021 (você aprova o rascunho) | `review/REVISAO-2026-09.md` |
| Login no console (admin-bff) — D6 | decidir **junto com o deploy real**: login de operador (OIDC/senha) ou console só atrás de VPN | `review/REVISAO-2026-09.md` §6 |
| Deploy real e medição de custo (G10, a tese do projeto — ADR-0008) | o próximo grande marco depois do lançamento; P2P já cortou o maior custo | `review/REVISAO-2026-09.md` |
| Resultado de partida sem outbox (G7) | só importa quando houver um jogo `queue_arena` real | `review/REVISAO-2026-09.md` |
| Limite de tentativas no login (G5) | antes de expor o identity publicamente | idem |
| Mais de um app Steam por deploy | quando um segundo jogo for para a Steam: `STEAM_APP_ID` por jogo no game-registry | `achievements-service/specs/019-steam-mirror` |
| SDK UPM | extrair do Folclore quando a camada C# (guia, seção 4) estabilizar | guia Unity, seção 8 |
| Abrir o repositório (Apache-2.0) + post técnico | no fim, como você pediu | `review/ESTRATEGIA-PRODUTO-2026-09.md` §4 |

## 7. Validar na sua máquina (a VM não conseguiu)

- **Imagens Docker:** `infra/docker/gen-secrets.sh`, depois
  `docker compose -f infra/docker/docker-compose.yml up -d --build` e `scripts/demo-host-world.sh` (e
  `TOPOLOGY=p2p scripts/demo-host-world.sh`). Suba num volume limpo (`down -v` antes) para as migrations novas
  rodarem — o compose agora monta os diretórios de migration inteiros.
- **Cloud-save:** `go test -tags=integration ./services/cloud-save-service/...` (a VM não baixou a imagem do MinIO).
- **Túnel do Pi 5:** `infra/agones/pi5/pi5-tunnels.sh install`.
- **Com o jogo:** `HOST_KILLED` com o LocalStack parado → o server recebe `Internal`, reenvia com o mesmo
  `event_id` e, com o LocalStack de volta, o invasor ganha a conquista.
