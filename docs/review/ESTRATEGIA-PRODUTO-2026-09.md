# Estratégia de produto — control plane × Folclore Souls (2026-09)

> Pedido do dono (2026-09-26): julgar a crítica de um colega ("em produção ninguém usaria isso"), avaliar se vale
> virar SaaS, pacote na Unity Asset Store ou open source, e como integrar com Steam/GOG no lançamento do Folclore
> Souls. Baseado na revisão (`REVISAO-2026-09.md`), no código deste repo e numa pesquisa de mercado feita na mesma
> sessão (fontes no fim; itens marcados **[não verificado]** vêm de conhecimento geral, não de fonte consultada).

## Veredito em cinco linhas

1. **Como caminho mais curto para lançar o Folclore Souls, o sistema está superdimensionado** — nisso o colega tem
   razão. Um estúdio pequeno lançando um Souls-like só na Steam usaria Steamworks (+ talvez Nakama ou EOS) e P2P.
2. **Como arquitetura, ele não está errado** — é o desenho que estúdios médios/grandes fazem sobre Agones, e a camada
   que o Photon não resolve (matchmaking de invasão por faixa, quadro de sinais) é exatamente a que a FromSoftware teve
   de construir sob medida.
3. **Para o Folclore**, a recomendação é **P2P com o host como autoridade sobre a rede da Steam** no lançamento, com o
   control plane fazendo o que só ele faz bem: login federado Steam, matchmaking de invasão/sinais, conquistas,
   telemetria. Servidor dedicado fica para modos onde ele se paga (PvP ranqueado, arena).
4. **Produto: open source (Apache-2.0) + vitrine, não SaaS.** Dois provedores gerenciados de hospedagem de jogo
   saíram do mercado em 2026; SaaS exige plantão 24/7 incompatível com um dev solo lançando um jogo. Asset Store só
   como kit enxuto e focado, depois do jogo lançado.
5. **SDK Unity: sim, mas extraído do Folclore quando a fase multiplayer voltar** — não antes. O contrato versionado
   (CHANGELOG + `buf breaking` no CI) já foi feito nesta revisão e é o pré-requisito.

## 1. A crítica do colega, com justiça

**"Você está usando o Photon?"** — Photon (Fusion/PUN) resolve *netcode + relay de salas*, cobrando por CCU (100 CCU
grátis; ~US$125/mês por 500 CCU, ~US$0,50/CCU acima de 2k). Não resolve o que este repo resolve: identidade própria,
matchmaking de invasão por faixa de progresso, quadro de sinais, sessão com papéis assimétricos, fan-out para
conquistas/analytics. As perguntas "Photon ou isto?" comparam camadas diferentes.

**Onde ele acerta:**
- **O Nakama existe, é em Go, é open source (Apache-2.0)** e cobre contas (incl. Steam), matchmaker, leaderboards,
  storage, partidas autoritativas. Um backend-dev de jogos pensa nele primeiro.
- **Steamworks e Epic Online Services dão de graça** login, conquistas, stats, leaderboards, cloud save, lobbies e
  relay (Steam Datagram Relay). Para um jogo só na Steam, boa parte dos serviços daqui duplica o que a Valve oferece.
- **Operar Agones + Kubernetes + SNS/SQS + 10 serviços é demais para uma pessoa** manter em produção com jogadores.

**Onde ele erra:**
- "Ninguém usaria" confunde escala com desenho: control plane próprio sobre Agones é o padrão para quem precisa de
  servidor autoritativo e matchmaking específico — o Agones existe (Google + Ubisoft) por isso.
- A camada específica do gênero (invasão por faixa, sinais, papéis, eventos por participante) **não existe pronta**
  em nenhum dos serviços de prateleira; em qualquer caminho alguém a escreve.
- **Depender de fornecedor deixou de ser risco teórico:** o Unity Multiplay Hosting foi descontinuado em
  31/03/2026 e a Hathora encerrou a hospedagem de jogos em maio de 2026 (jogos como Stormgate perderam o
  multiplayer na transição). Self-hosting sem taxa por CCU ganhou argumento.
- **O objetivo declarado é aprender Go e mostrar engenharia.** Nisso o projeto já entrega: hexagonal real,
  idempotência na mesma transação do efeito, contratos versionados, observabilidade, 30 ADRs.

**Resumo honesto:** certo como estudo, portfólio e base para o gênero; superdimensionado como o caminho mais curto
para colocar o Folclore na Steam. As duas coisas cabem juntas se o lançamento não depender do caminho mais caro.

## 2. Como os Souls fazem — e o que isso muda para o Folclore

- **FromSoftware (DS3, Elden Ring):** a sessão é **P2P com o host como autoridade**. Os servidores da From fazem
  matchmaking (nível + maior upgrade de arma, senha como alternativa), quadro de sinais, mensagens, manchas de
  sangue e fantasmas. No PC o transporte passa pela Steam. Os Souls convivem com trapaça do host em troca de custo zero
  de servidor de jogo.
- **O Folclore hoje** usa servidor dedicado autoritativo por mundo (Agones). É mais robusto que o modelo da From —
  e muito mais caro: um mundo por 0,5–1 vCPU num provedor sob demanda custa ~US$0,03–0,07 por hora de mundo aberto;
  200 mundos simultâneos dariam alguns milhares de dólares por mês **[estimativa]**. Cluster próprio barateia, mas
  exige operação.
- **Recomendação para o lançamento:** topologia **P2P host-autoritativo** (o cliente do host roda a simulação, como a
  From) sobre **Steam Networking Sockets / Steam Datagram Relay** (grátis, esconde o IP do host). O control plane
  continua dono de: login (federado com a Steam), matchmaking de invasão e sinais, conquistas, analytics. O
  `OpenWorld` deixa de alocar GameServer quando o modo é P2P e devolve, no lugar do endereço do servidor, o
  identificador de conexão do host na Steam.
- O arquétipo já prevê isso: o perfil tem `topology` (`dedicated`/`p2p`, ADR-0023) e o follow-up #10 (alocação sob
  demanda) é o mesmo problema visto do outro lado. Dedicado continua existindo para modos onde vale pagar (PvP
  ranqueado, arena, anti-cheat).
- **Decisão que é do jogo, não daqui:** o netcode do Folclore (NGO) teria de rodar com transporte Steam no modo
  host. O R&D de topologia DS/ER-like já está aberto no repo do jogo (ADR-0002/0010 de lá) — a decisão lá destrava a
  ADR de topologia aqui (rascunho B abaixo).

## 3. Steam, GOG e as outras lojas

| Necessidade | Opção | Como liga no control plane |
|---|---|---|
| **Login** | Steam: `ISteamUser::GetAuthTicketForWebApi("<identidade>")` no cliente; backend chama a Web API `ISteamUserAuth/AuthenticateUserTicket` (publisher key) e recebe o SteamID64, dono do app (Family Sharing) e banimentos VAC/publisher | identity ganha `POST /sessions/steam {ticket}` → valida o ticket → cria/vincula a conta ao SteamID → emite o JWT de sempre. Nada muda nos outros serviços (rascunho A) |
| Login em outras lojas | GOG Galaxy: `RequestEncryptedAppTicket` + verificação no backend. EOS Connect: identidade entre lojas (Steam/GOG/Epic → um `ProductUserId`), grátis | mesmo endpoint por provedor; cada provedor vira um vínculo da mesma conta |
| **Conquistas** | Steam: `ISteamUserStats::SetAchievement` no cliente, ou a Web API `SetUserStatsForGame` pelo servidor | achievements daqui continua a fonte da verdade (regras visuais, stats) e **espelha** o desbloqueio na Steam por um consumidor novo do fan-out — o jogador vê a conquista da Steam, o operador ganha o motor de regras |
| Leaderboard | Steam leaderboards ou o daqui | para um Souls, ranking é secundário; manter o daqui só se houver modo competitivo |
| **Cloud save / progresso** | Steam Cloud (grátis, automático) | para PC-only, **Steam Cloud basta** para o save do mundo; o cloud-save daqui só se paga com cross-store/cross-platform |
| Transporte | Steam Networking Sockets + SDR (P2P e também servidores dedicados, com proteção de IP) | nada no control plane; o `OpenWorld` só repassa o identificador de conexão |
| Lobby | Steam Lobbies | desnecessário: o matchmaking de invasão/sinais daqui é o "lobby" do gênero |

**Recomendação:** federar a identidade na Steam **antes do lançamento** (é o que torna o backend utilizável com
jogadores reais), espelhar conquistas na Steam, delegar save/transporte à Steam e manter aqui só o que é específico
do gênero. GOG/Epic entram pelo mesmo desenho de provedor quando o jogo for para lá.

## 4. SaaS, Asset Store, open source ou estudo?

| Caminho | Prós | Contras | Recomendação |
|---|---|---|---|
| **SaaS** | receita recorrente | mercado de commodity (Edgegap, GameLift, PlayFab) e que acabou de perder dois players; plantão 24/7, SLA, DDoS, suporte; mata o tempo do jogo | **Não** |
| **Asset Store (SDK + kit de self-host)** | canal direto com devs Unity | ticket baixo (US$30–100); suporte a quem sobe Kubernetes é pesado; o kit completo é grande demais para o público da loja | Só depois do lançamento, como produto **enxuto e focado** ("Souls-like online kit": invasão, sinais, papéis — sem Agones obrigatório) |
| **Open source (Apache-2.0) + vitrine** | custo quase zero; mostra o código (o objetivo 3); atrai issues e contribuições; é o que o Nakama fez para crescer | monetização só se houver adoção (open-core, suporte, hospedagem) — não conte com isso | **Sim** — abrir o repo quando o README e a demo estiverem prontos (já estão), com um post técnico "o que um Souls-like online precisa de um backend" |
| Só estudo | sem compromisso | desperdiça a parte que já é vitrine | — |

**Sequência recomendada:** (1) lançar o Folclore; (2) abrir o repo como open source com a narrativa do gênero; (3) só
então medir se há demanda (estrelas, issues, estúdios pedindo) para um kit pago ou suporte.

## 5. SDK Unity e manutenção do contrato

- **Vale ter SDK?** Sim, porque hoje cada jogo gera os stubs C# e reescreve login, retry, reenvio com `event_id`,
  metadata de auth e leitura do manifest. Mas **extraído do Folclore** (o único cliente real), não escrito no vácuo —
  a própria lição do dono sobre genérico sem caso real.
- **Forma:** pacote UPM (`com.<org>.controlplane`, `Runtime/`, `Samples~/`, asmdef) distribuído por git URL com tag
  e depois OpenUPM. gRPC no Unity via `Grpc.Net.Client` + **YetAnotherHttpHandler** (Cysharp; o Grpc.Core antigo está
  descontinuado); gRPC-Web só se um dia houver WebGL.
- **Contrato (feito nesta revisão):** `docs/contracts/CHANGELOG.md` (Keep a Changelog + SemVer, 1.0.0 → 1.2.0) e o job
  `buf breaking` no CI, que falha em mudança incompatível dos `.proto`. Próximo passo quando o SDK existir: o SDK
  declara a faixa de contrato suportada e o servidor expõe a versão do contrato na metadata.

## 6. Roadmap proposto (produto)

| # | Item | Depende de | Por quê |
|---|---|---|---|
| 1 | **Login federado Steam** no identity (rascunho A) — ✅ **feito** (spec `identity/005-steam-login`, validado com Steam falsa) | publisher key do Steamworks para validar contra a Steam real | é o que torna o backend utilizável com jogadores reais |
| 2 | **Topologia P2P host-autoritativa** no `host_world_dropin` (rascunho B) — ✅ **backend feito** (ADR-0032, spec `session/031`, demo `TOPOLOGY=p2p`) | netcode no repo do jogo (NGO + transporte Steam no modo host) | corta o maior custo do lançamento |
| 3 | Espelhar conquistas na Steam (consumidor do fan-out) | 1 | o jogador vê a conquista onde espera |
| 4 | Virar o `SESSION_AUTH_MODE` para `enforce` | jogo mandar bearer e chave de servidor | fecha o buraco que a ADR-0030 deixou em `warn` |
| 5 | SDK UPM extraído do Folclore | fase multiplayer do jogo | reuso real |
| 6 | Abrir o repo (Apache-2.0) + post técnico | 1–2 idealmente | vitrine e reconhecimento |

## Rascunhos de ADR

> Atualização 2026-09-26: o dono aprovou as recomendações. **A** virou a
> [ADR-0031](../adr/0031-federated-identity.md). **B** virou a spec em rascunho
> `services/session-service/specs/031-p2p-topology/spec.md` (aguarda aprovação e a decisão de netcode no jogo).

**A — Identidade federada (Steam primeiro).** O identity aceita um ticket de loja (`POST /sessions/{provider}`),
valida com a Web API do provedor, vincula o ID externo a uma conta (tabela `external_identities(provider,
external_id, account_id)`; primeira vez cria a conta com o nome do perfil), e emite o mesmo JWT. Alternativas: EOS
Connect como identidade única (menos código, mais uma dependência); Nakama inteiro (troca a stack). Recomendado:
provedor por provedor, Steam primeiro, EOS só se houver cross-store.

**B — Topologia `p2p` no `host_world_dropin`.** Com `topology: p2p`, o `OpenWorld` não aloca GameServer: registra
o mundo (slots, faixa, senha, modo), guarda o identificador de conexão do host (ex.: SteamID para Steam Networking) e
o devolve no `JoinWorld`/`Invade`. O connect token continua existindo (o host valida quem entra, como o server faz
hoje). Os reports vêm do cliente do host — mesma confiança que a From dá ao host; a credencial é um token de sessão
emitido no `OpenWorld` para o host (a chave de servidor da ADR-0030 não pode ir num cliente). Alternativa: manter só
dedicado (custo), ou alocar sob demanda (follow-up #10, meio-termo).

## Fontes

- Photon Fusion — preços: https://www.photonengine.com/fusion/pricing
- Unity Multiplay Hosting — encerramento: https://status.unity.com/info_notices/362941 ·
  https://gameye.com/blog/unity-multiplay-shutdown-migration-options/
- Hathora — saída do mercado de hospedagem: https://gamesbeat.com/hathora-acquired-will-exit-game-infrastructure-biz-and-hand-over-customers-to-nitrado/
- Nakama (Apache-2.0): https://github.com/heroiclabs/nakama · preços: https://heroiclabs.com/pricing/
- AWS GameLift Anywhere — preços: https://aws.amazon.com/gamelift/servers/pricing/anywhere-pricing/
- Edgegap — preços: https://edgegap.com/resources/pricing
- Steam — autenticação: https://partner.steamgames.com/doc/features/auth ·
  https://partner.steamgames.com/doc/webapi/isteamuserauth
- Elden Ring — matchmaking: https://eldenring.wiki.fextralife.com/Multiplayer_Coop_and_Online ·
  https://www.gameskinny.com/tips/how-multiplayer-matchmaking-works-in-elden-ring/
- YetAnotherHttpHandler (gRPC no Unity): https://github.com/Cysharp/YetAnotherHttpHandler
- Keep a Changelog: https://keepachangelog.com · SemVer: https://semver.org
- **[não verificado]**: detalhes de EOS, PlayFab, Steam Cloud/SDR/leaderboards e GOG Galaxy vêm de conhecimento geral
  do ecossistema, não de fonte consultada nesta sessão.
