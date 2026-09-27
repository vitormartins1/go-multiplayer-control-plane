# Spec: 091-p2p-console

**Serviço:** admin-ui (+ passthrough no session-service e no admin-bff)
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** pedido do dono após `session/031` (p2p) e `session/032` (heartbeat) — "ver se cabe mais coisa específica do p2p"

## Contexto

O console já mostra "P2P · host" no lugar do servidor e um selo P2P no modo. Falta o que o operador precisa para
**operar** um mundo p2p: com quem o jogo conecta (o id de conexão do host), se o mundo está vivo (heartbeat) e o
aviso de que os eventos vêm do host. E o mesmo sinal de vida serve para os mundos dedicados.

## Comportamento

1. O JSON de sessão do session-service (e o passthrough do BFF) **deve** trazer `hostConnection` (p2p) e
   `lastHeartbeatAt` (quando houver) — aditivos.
2. **Rooms:**
   - filtro **Hospedagem** (Todas / Dedicado / P2P), visível só quando há mundo p2p na lista;
   - coluna **Hospedagem** (antes "Server"): endereço do GameServer em dedicado; selo `P2P` + id do host em p2p;
   - no estado de mundo aberto, um indicador de **sinal**: `vivo` (heartbeat ≤ 45 s), `atrasado` (≤ 90 s),
     `sem sinal` (> 90 s) e `sem heartbeat` (o jogo não manda — fecha aos 30 min);
   - painel expandido de mundo p2p: hospedagem, conexão do host, último heartbeat e um aviso de confiança
     ("os eventos deste mundo vêm do jogo do host");
   - modal **Encerrar**: o texto diz o efeito certo por topologia (dedicado libera o GameServer; p2p invalida o
     world token e o host recebe `NotFound` no próximo heartbeat).
3. **Detalhe do jogo:** o selo de cada modo p2p mostra `P2P`.
4. **Fleet:** jogo cujos modos são todos p2p não pede fleet — o cadastro mostra "P2P — sem frota".
5. **Sessões do jogo** (aba): a mesma coluna/indicador de Rooms.

## Critérios de aceite

- Unit (Vitest) das regras puras: `hostingOf`, `heartbeatStatus` (limiares e ausência).
- `tsc`, `vitest`, `vite build` verdes; testes Go do passthrough.
- Captura com Playwright de Rooms com um mundo p2p vivo, um dedicado e um p2p sem sinal.
