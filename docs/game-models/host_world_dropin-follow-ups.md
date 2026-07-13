# Follow-ups — arquétipo `host_world_dropin` (ADR-0023)

> **Propósito:** backlog consolidado das **evoluções deliberadamente adiadas** ao longo das fatias H0–H2 do `host_world_dropin`. Cada item foi cortado do escopo por foco (control plane primeiro) ou por depender de validação no data plane. Aqui ficam reunidos com **contexto suficiente para retomar** — o quê, por que foi adiado, onde mexer, e o que destrava.
>
> **Quando voltar:** a validação no data plane (`unity-host-world-dropin-data-plane`, Unity) **já está substancialmente em andamento** — repo real, Fases 0–4 done (mundo do host, phantoms join-in-progress, invasão+sinais com senha, combate/boss). O follow-up #1 (senha) já foi **confirmado por esse feedback real**. **Correção 2026-07-13:** o #6 (stats) NÃO foi confirmado — o campo `stats` nem existia no proto/envelope de `ReportParticipationEvent` até `session-service/026` (achado ao especificar `analytics-service/011`); a alegação anterior era otimista/incorreta (ver nota na seção 6). Continue priorizando por evidência: o que o gameplay real mostrar que falta primeiro (Fase 5 e2e-orquestrado k3d→Pi 5 ainda pendente; confirmar se o Dedicated Server já preenche `stats` agora que o campo existe).
>
> **Fontes:** a seção "Aberturas" do [`host_world_dropin-roadmap.md`](./host_world_dropin-roadmap.md) e o "Fora de escopo" de cada spec (`session/011–013`, `matchmaking/006–007`, `achievements/009`, `analytics/004`). A tabela de estado vive no [README](./README.md).

## Estado do arco (o que já está na `main`)

Control plane **ponta a ponta**, validável com server mockado (`static` allocator + cliente de teste):
declarar → **abrir mundo** (H0, `session/011`) → **descobrir/injetar por papel** (H1a invasão `session/012`+`matchmaking/006`; H1b co-op por sinais `matchmaking/007`) → **reportar eventos por participante** (H2 produtor `session/013`) → **colher** (H2 sinks `achievements/009` + `analytics/004`).

**H3 (surfacing no console) também já está feito** (`session/014` + `admin-bff/014` + `admin-ui/020` — ver "Próximo passo estrutural" no fim deste doc, que já registrava isso; esta linha estava desatualizada). Falta só o **contrato + e2e real** cross-repo (Agones + Unity) e os itens abaixo.

---

## 1. Play-with-friends por senha (o mais pedido) — ✅ FEITO (2026-07-09)

- **O quê:** casar co-op/invasão por **senha** que **ignora** as faixas de nível/arma (ficha 01 §4 — "se as senhas batem, casam"). A flag `PasswordEnabled` já existe no perfil (game-registry), mas nenhum serviço a consome.
- **Por que adiado:** manter H1a/H1b focadas na descoberta por faixa. Senha é aditiva mas atravessa produtor+matchmaking+token.
- **Onde mexe:**
  - `session/012` `OpenWorld` passa a receber a senha do host (novo atributo, ao lado de soul level/weapon tier); o `InvadableWorld`/diretório carrega o hash.
  - `matchmaking/006` (`Invade`) e `007` (`ListSigns`/`PostSummonSign`): quando há senha e ela bate → **bypassa** `Eligible`/`CoopEligible`.
  - Decidir armazenamento da senha (hash vs plaintext efêmero) e se vira um pool/quadro separado.
- **Destrava:** jogar com amigos ignorando o range — o caso de uso social nº 1 do arquétipo.
- **Fonte:** "Fora de escopo" de `session/012`, `matchmaking/006`, `matchmaking/007`.
- **Evidência (2026-07-08):** o Capoeira Souls tem o modo `batizado` cadastrado com `passwordEnabled=true` e faixas desligadas (catálogo em `unity-host-world-dropin-data-plane/docs/MODES.md`) — o modo existe e espera este follow-up para funcionar de verdade.
- **Implementado (`session/020-password-world-matching`, `matchmaking/011-password-matching`):** hash SHA-256 da senha gravado no `OpenWorld`; `Invade`/`PostSummonSign`/`ListSigns` bypassam a faixa de nível/arma quando a senha do candidato bate com a do mundo. Destrava o modo `batizado` do Capoeira Souls de verdade.

## 2. Pools / área / região no quadro de sinais

- **O quê:** o sinal da ficha vive num **pool ligado a uma área do mapa**; hoje o quadro (`matchmaking/007`) é **por jogo** (sem dimensão de área). Idem invasão.
- **Por que adiado:** área/região só faz sentido com o mapa real do data plane.
- **Onde mexe:** `PostSummonSign`/`ListSigns` ganham `area`/`region`; o filtro de compatibilidade adiciona a dimensão; o store indexa por pool.
- **Destrava:** invocação/invasão localizada por região do mundo (menos latência, mais fidelidade ao original).
- **Fonte:** "Fora de escopo" de `matchmaking/007`.

## 3. Store de sinais distribuído (Redis + pub/sub)

- **O quê:** o quadro de sinais (`matchmaking/007`) é **em memória** com canal de entrega por sinal (o `PostSummonSign` streaming acorda o aliado). Multi-instância exigiria mover o store para Redis **e** um pub/sub para a entrega cross-instância (um canal Go não atravessa Redis).
- **Por que adiado:** MVP single-instance; memória é trivial e suficiente para validar.
- **Onde mexe:** `internal/service/summoner.go` (o store interno) → adapter Redis + assinatura pub/sub para o `SUMMONED`. Candidato a **ADR** (introduz dependência nova no matchmaking, hoje stateless).
- **Destrava:** escalar o matchmaking horizontalmente sem perder sinais/entregas.
- **Nota de design:** ver GO-LEARNINGS #29 (por que o canal fechado-vs-enviado prende o modelo à memória).
- **Fonte:** "Fora de escopo" de `matchmaking/007`; decisão registrada no `plan.md` D1.

## 4. `eventId` único por ocorrência (contar N bosses na mesma sessão) — ✅ FEITO (2026-07-09)

- **O quê:** o produtor (`session/013`) publica `participation_event` **sem** um id único por ocorrência. Consequência: os sinks (`achievements/009`, `analytics/004`) fazem idempotência por `(sessionId, playerId, event)` → **uma contagem por tipo de evento por sessão**. "Ajudou N bosses como co-op" hoje = **N sessões**, não N bosses numa sessão.
- **Por que adiado:** o combinado da H2 (o roadmap escolheu essa chave); cobre os casos-alvo (venceu como invasor = um `HOST_KILLED`).
- **Onde mexe:** adicionar `eventId` (uuid) ao envelope em `session/013` (publisher) → os sinks trocam a chave de idempotência para `eventId` (migrations `processed_participations` ganham a coluna/chave).
- **Destrava:** contagem fiel de ocorrências múltiplas do mesmo evento numa sessão.
- **Fonte:** "Fora de escopo"/OBSERVAÇÃO de `session/013`, `achievements/009`, `analytics/004`.
- **Implementado (`session/021`, `achievements/012`, `analytics/007`):** o `eventId` é gerado pelo **data plane** (Unity, spec 004), não pelo session — gerar na recepção quebraria a garantia de retry (um retry do client criaria um `eventId` diferente pra mesma ocorrência, achado durante o design desta fatia). `session/021` só repassa o campo pro envelope. `achievements/012`/`analytics/007` trocam a PK de `processed_participations` pra `event_id`, com backfill determinístico (`session_id:player_id:event`) nas linhas existentes e fallback pra quem ainda não manda o campo. Migrations 007 (achievements) e 006 (analytics).

## 5. Dimensão `outcome` na agregação de analytics — ✅ FEITO (2026-07-09)

- **O quê:** `analytics/004` agrega participações por `(role, event)` — **sem** `outcome`. Não distingue "invasões bem-sucedidas vs. falhas" (`HOST_KILLED`+SUCCESS vs. FAIL).
- **Por que adiado:** conter cardinalidade até o funil pedir.
- **Onde mexe:** `game_participation_rollup` ganha a coluna `outcome` na PK; `ApplyParticipation` e o read (`RoleParticipations`) propagam.
- **Destrava:** funil de invasão com taxa de sucesso por papel (H3).
- **Fonte:** "Fora de escopo" de `analytics/004`.
- **Implementado (`analytics/007`, mesma migration 006 do item 4):** `game_participation_rollup` ganha `outcome` na PK (`game_id, role, event, outcome`); linhas antigas ficam sob `outcome=''` (não reconstituível sem os eventos originais). Evidência real: a spec 004 do Unity introduz `HOST_KILLED`·`FAIL` (o boss mata o host sem invasor por perto) — sem esta dimensão, ficaria indistinguível de um `HOST_KILLED` normal no rollup.

## 6. Critério de conquista por **stat** para participação — ✅ FEITO no control plane (2026-07-13)

- **O quê:** `achievements/009` só usa critério por **event_type** (`invasion_won`, etc.). Não há "cause N mortes como invasor" (critério por stat), porque o `participation_event` não carrega stats.
- **Por que adiado:** o produtor não manda stats no evento de participação.
- **Onde mexe:** `session/013` incluir stats no `ParticipationEvent`/envelope; `achievements/009` reusar `findByStatKey`/`recordStatProgress` (A1) no caminho de participação.
- **Destrava:** conquistas cumulativas por métrica no host_world (paridade com o queue_arena A1).
- **Fonte:** "Fora de escopo" de `achievements/009`.
- **Implementado (`achievements/018-participation-stats`, 2026-07-10):** achado durante o estresse-teste do motor de regras com o jogo "Achievements Test" — `Service.ApplyParticipationEvent` soma via `Facts.WithStats`; repositório roda `recordStatProgress` na mesma transação idempotente do evento — espelha `ApplyMatchResult` dos outros 3 arquétipos. `game-registry/011` fechou o gap irmão (`HostWorldDropInParams` ganhou `ScoringStats`, que faltava pro canvas parar de marcar stat de HWD como "drift").
- **Correção 2026-07-13 (achado ao especificar `analytics-service/011-raw-stat-capture`):** a spec 018 acima só implementou o lado de RECEBER — o `session-service` nunca enviava o campo `stats` (nem no proto, nem no domínio, nem no envelope publicado no SNS). Na prática, `Stats` chegava sempre `nil` nesse caminho e **nenhum achievement por stat de host_world_dropin jamais disparou de verdade** desde 07-10 até agora, silenciosamente. Fechado por `session-service/026-participation-event-stats`: `ReportParticipationEventRequest` ganhou `map<string, int64> stats = 7`, mesmo padrão de `ReportResult`. Validado ao vivo: `deaths_1` desbloqueou de verdade pela primeira vez (grpcurl, jogo "Achievements Test"). **Pendente:** confirmar que o Dedicated Server Unity (`unity-host-world-dropin-data-plane`) preenche `stats` na chamada real — o campo só existe no control plane desde 07-13, então não há como ter sido validado com gameplay real ainda.

## 7. Seed de catálogo de conquistas de invasão/co-op

- **O quê:** as conquistas com `event_type` `invasion_won`/`coop_boss_helped`/`world_joined` **funcionam** (data-driven), mas não há **seed** de exemplo — o operador precisa cadastrar pelo console.
- **Por que adiado:** é conteúdo/dados, não código; melhor com um jogo real de referência.
- **Onde mexe:** migration de seed no achievements (como o sandbox atual) ou cadastro via console.
- **Fonte:** "Fora de escopo" de `achievements/009`.

## 8. Participações no `Overview` cross-game

- **O quê:** `analytics/004` expõe participações só no **detalhe do jogo** (`GameAnalytics.roleParticipations`), não no `Overview` cross-game.
- **Por que adiado:** evitar cardinalidade no panorama; o detalhe cobre o funil.
- **Onde mexe:** `Overview` (repo + DTO) agrega participações por jogo.
- **Fonte:** "Fora de escopo" de `analytics/004`.

## 9. Observabilidade fina do funil (contadores/gauges por jogo)

- **O quê:** vários contadores/gauges foram adiados para a **H3** (quando o dashboard do funil ganha sentido visual): `invadable_worlds{game}` (session/012), `summon_signs_active` com label por jogo (matchmaking/007 — hoje sem label, cardinalidade), `participation_events_consumed_total{event}` / `participations_ingested_total{event}` (sinks).
- **Por que adiado:** gauge/contador com label por jogo pede gestão de cardinalidade e só compõe o funil junto do surfacing.
- **Onde mexe:** os `main.go` dos serviços + dashboards Grafana.
- **Destrava:** o funil de invasão/co-op no console (H3).
- **Fonte:** `plan.md` de `session/012` (D5), `matchmaking/007` (D6), `analytics/004` (D6).

## 10. Alocação do dedicated server **sob demanda** (a economia real)

- **O quê:** a H0 aloca um Agones GameServer **no `OpenWorld`** (mundo sempre num server — simples, mais caro). A evolução é alocar **sob demanda** só quando o mundo abre para multiplayer (1º co-op/invasão) e liberar ao esvaziar (barato, exige handoff de estado).
- **Por que adiado:** validar o fluxo ponta a ponta e **medir** antes; o handoff introduz latência/complexidade.
- **Onde mexe:** `session` (ciclo de vida do server no mundo host_world) — decisão #2 do roadmap. Candidato a **ADR** quando escolhido.
- **Destrava:** custo viável de mundos solo-online em escala (a tese econômica, ADR-0008).
- **Fonte:** roadmap "Aberturas", decisão de modelagem #2.

## 11. Ordenação estável do diretório de invadíveis — ✅ FEITO (2026-07-08)

- **O quê:** `session/012` `ListInvadableWorlds` **não garante ordem** (iteração de mapa Go é aleatória). O matchmaking filtra e tenta em ordem de retorno.
- **Por que adiado:** o `matchmaking/006` não depende de ordem.
- **Onde mexe:** ordenar por `OpenedAt` no `ListInvadableWorlds` se quisermos estabilidade/fairness.
- **Fonte:** `plan.md` de `session/012` (D2).
- **Implementado (branch `chore/host-world-dropin-h3-docs-e-ordenacao`):** `ListInvadableWorlds` agora ordena por `OpenedAt` asc (mundo mais antigo primeiro) antes de montar a resposta — dois `JoinWorld` concorrentes na mesma faixa passam a tentar os mundos na mesma ordem determinística. Teste novo (`TestListInvadableWorlds_OrdenaPorOpenedAtAsc`) abre 3 mundos fora de ordem cronológica via `WithClock` e prova que a saída reflete `OpenedAt`, não a ordem de criação/iteração do mapa.

## 12. Revalidar `role`/`outcome` contra a semântica de papel

- **O quê:** `session/013` valida só o **vocabulário** do evento; não checa "invasor não pode reportar BOSS_DEFEATED". O server do mundo é autoritativo.
- **Por que adiado:** confiar no server (autoridade neutra); regra de papel é refinamento.
- **Onde mexe:** `validateParticipation` no service do session, se o modelo de confiança mudar.
- **Fonte:** "Fora de escopo" de `session/013`.

## 13. Params novos para o arquétipo (propostos pelo catálogo de modos do Capoeira Souls)

- **O quê:** o exercício de multi-modo no data plane (2026-07-08, `unity-host-world-dropin-data-plane/docs/MODES.md`) propôs params colhidos dos jogos de referência: `invasionRequiresCoop` (ER: host solo não é invadível), `hostDeathPolicy` (`close_world`|`respawn` — Seamless Co-op), `invasionCooldownSeconds`, `bossPowerScalingPerPhantom` e `passwordScaling` (data plane, declaráveis por modo), papel "cinza"/`extraRoles` (Mound-Maker; `invaderAsBoss` do Old Monk), `worldScene`/`mapId`.
- **Onde mexe:** `HostWorldDropInParams` (game-registry) + consumidores conforme o dono (control plane: diretório/matchmaking; data plane: só declaração no blob).
- **Destrava:** os dials de risco/social que diferenciam os Souls reais; adotar **por evidência**, um por vez.

## 14. Bloco passthrough `worldParams` no perfil (registry descarta chaves desconhecidas) — ✅ FEITO (2026-07-08)

- **O quê:** o registry decodifica `params` num struct tipado — **chave desconhecida é descartada em silêncio** no PUT. O data plane não tem como declarar params próprios por modo (o princípio do manifest "leia só o que conhece" vale na leitura, mas a escrita filtra tudo).
- **Onde mexe:** game-registry (DTO/validação/persistência): um bloco livre `worldParams` (JSON cru, sem validação de schema) preservado no snapshot/manifest; ou, no mínimo, **rejeitar** chave desconhecida em vez de descartar (surpresa silenciosa de operador).
- **Destrava:** params de gameplay por modo sem mudança de schema no registry (ex.: os do item 13 com dono "data plane").
- **Fonte:** descoberta operacional de 2026-07-08 (cadastro dos modos do Capoeira Souls).
- **Implementado (branch `feat/game-registry/worldparams-passthrough`):** escolhida a opção do **passthrough** (não a rejeição) — chave fora do vocabulário tipado de `host_world_dropin` some do struct decodificado mas sobrevive crua em `MultiplayerProfile.ExtraParams` (novo pacote `internal/jsonparams`, `Extract`/`Merge`) e é reinjetada no `params` de volta na leitura (HTTP GET e persistência JSONB) — chave tipada sempre manda, extra nunca sobrescreve. **Zero mudança no session-service ou no contrato do data plane**: `ParamsJSON`/`params_json` já eram bytes crus repassados verbatim (`registryclient.ModeProfile`), então a chave extra já chega ao Unity dentro do `params_json` de sempre — é só ler quando alguma spec precisar (ex.: item 13). Escopo só `host_world_dropin` por ora (outros arquétipos têm o mesmo problema, mas sem evidência de demanda ainda — replicar por evidência, mesmo princípio deste doc).

## 15. Phantom morto sumia da sala do admin (`players[]` como presença) — ✅ FEITO (2026-07-12)

- **O quê:** na bateria real do data plane (Agones/k3d, 2026-07-12), o operador viu o coop invocado por sinal "nunca aparecer" na sala do admin. Investigação com repro limpa (grpcurl ponta a ponta) mostrou que o caminho dos sinais **estava correto** (`SummonSign → JoinWorld → Players`); a causa real: `DIED`/`LEFT` chamavam `removeFromWorld`, que apagava o phantom **de `Players` além do slot** — e os bots coop morriam rápido pro boss. O jogador sumia da sala como se nunca tivesse entrado.
- **Implementado (session-service, 2026-07-12):** `Players` de host_world virou lista de **PARTICIPANTES** (acumula no `JoinWorld`, com dedupe na re-entrada pós-morte; rollback de token remove só quem nunca entrou) — presença/capacidade continua sendo papel dos `RoleSlots`. `removeFromWorld` só libera o slot. Validado por teste (176 no serviço) e ao vivo (DIED → slot 0, participante permanece na sala).
- **Evolução natural:** spec **`session/025-session-history`** (Proposed) — participantes com PAPEL no domínio + persistência das sessões fechadas (hoje 100% in-memory) + aba History no player e fechadas persistidas na aba Sessions do jogo.

---

## Aberturas maiores (roadmap-level, além do arco atual)

Declaradas no roadmap, fora do H0–H3; cada uma é investimento próprio (provável ADR/repo):

- **Topologia `p2p`** (host como listen-server + relay/TURN): afinidade estrutural do arquétipo com P2P (ficha §topologia). Slot no perfil (`topology: p2p`); dedicated primeiro (autoridade neutra, ADR-0008).
- **PvPvE simultâneo / "modo cinza" (Mound-maker):** papel não-binário; refina H0/H2 depois do básico co-op+invasor.
- **Covenants/facções, mensagens no chão:** social **assíncrono** persistente — fora do escopo (mais perto de UGC/analytics que de session/matchmaking).

## Próximo passo estrutural (não é "follow-up", é a continuação planejada)

- **H3 — surfacing no console** (`admin-bff` + `admin-ui`): Rooms por papel/slots, participações por papel, quadro de sinais. Consome o `roleParticipations` (analytics/004) e os slots (session/011).
- **Contrato do data plane** (`host_world_dropin-dataplane-contract.md`) + **e2e real** (Agones + Unity encarnando o mundo): a cauda cross-repo que consome tudo o que H0–H2 entregaram.
