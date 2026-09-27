# ADR-0032: Topologia p2p no `host_world_dropin` — o jogo do host hospeda, o control plane continua dono do resto

**Status:** Accepted (rascunho B da estratégia de produto 2026-09, aprovado pelo dono em 2026-09-26)
**Data:** 2026-09-27
**Serviço(s) afetado(s):** session-service, matchmaking-service, game-registry, admin-bff, admin-ui, data plane (Unity)
**Relaciona:** ADR-0023 (perfil com `topology`), ADR-0030 (autenticação — ganha um quarto chamador), ADR-0026/0029
(Agones, que continua valendo para os modos dedicados), spec `session/031`

---

## Contexto

Todo mundo aberto do `host_world_dropin` aloca um GameServer. O custo cresce com as horas de mundo aberto, mesmo
com o host jogando sozinho a maior parte do tempo. O Folclore Souls vai lançar na Steam, e o gênero tem um modelo
conhecido: nos Souls da FromSoftware o **cliente do host** roda a simulação; os servidores da From só fazem
matchmaking, sinais e mensagens, e o transporte vai pela Steam. O perfil já reservava `topology: p2p` (ADR-0023),
mas o registry recusava o valor.

## Alternativas Consideradas

### Opção A: manter só dedicado
- **Prós:** autoridade neutra, sem trapaça do host; nada muda.
- **Contras:** o maior custo do lançamento continua lá; exige operar Agones desde o dia 1.

### Opção B: alocar dedicado só quando entra o primeiro phantom (follow-up #10)
- **Prós:** o host sozinho não custa servidor.
- **Contras:** migrar o mundo do cliente para o servidor no meio da sessão é difícil no netcode; ainda paga servidor
  em toda invasão.

### Opção C (escolhida): `topology: p2p` no `host_world_dropin`
- O `OpenWorld` não aloca: registra o mundo com o `host_connection` do host (ex.: SteamID64) e devolve um
  `world_token`. Quem entra recebe o `host_connection` no lugar do endereço do servidor.
- O jogo do host faz o papel do Dedicated Server: valida o connect token (claim `server` = `host_connection`) e
  reporta os eventos com `x-world-token`.
- **Prós:** custo zero de servidor de jogo; o transporte (Steam Networking Sockets / SDR) é grátis e esconde o IP do
  host; nada muda em matchmaking, conquistas, analytics e admin — os dados chegam pelo mesmo caminho.
- **Contras:** o host pode mentir nos eventos (mesma confiança que a From dá ao host); a latência depende do host;
  se o host cair o mundo acaba (sem migração de host).

## Decisão

Opção C, só no `host_world_dropin`. Os outros arquétipos seguem só `dedicated`, e o dedicado continua disponível
para os modos em que valer pagar pela autoridade neutra (PvP ranqueado, arena).

Por que um `world_token` e não o JWT do host: o JWT do identity dura 15 minutos e um mundo dura horas; o token
opaco nasce no `OpenWorld`, vale só para aquele mundo e morre quando ele fecha. O session guarda só o SHA-256.

## Consequências

- **Positivas:** o lançamento pode rodar sem cluster de game servers. O admin mostra "P2P · host" na coluna de
  servidor e um selo P2P no modo; Frota (Agones) passa a importar só para os modos dedicados.
- **Negativas / riscos:** conquistas e analytics de mundos p2p dependem da honestidade do host. Mitigações baratas,
  fora desta ADR: conquista sensível só com evento de mais de um participante; separar números por topologia.
- **Pendências:** o netcode do jogo (NGO com transporte da Steam no modo host) é decisão do repo do jogo. (O histórico
  persistido passou a guardar a topologia e a conexão do host na migration `004`.)
- **Contrato:** aditivo (1.4.0, `docs/contracts/CHANGELOG.md`).
