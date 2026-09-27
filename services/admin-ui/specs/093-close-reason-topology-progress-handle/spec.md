# Spec: 093-close-reason-topology-progress-handle

**Serviço:** admin-ui
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** gravação dos vídeos de feature. Fatia pequena: dois rótulos.

## Contexto

1. Um mundo P2P que fecha por falta de heartbeat aparecia como **"Servidor caiu"**, mas no P2P não há servidor:
   quem caiu foi o jogo do host.
2. O progresso de uma conquista listava o jogador pelo UUID (`714ae935…`); o resto do console mostra o handle.

## Comportamento

- O motivo `abandoned` SHALL ser rotulado conforme a topologia da sessão: **"Servidor caiu"** no dedicado,
  **"Host caiu (sem heartbeat)"** no P2P e **"Servidor ou host caiu"** quando a topologia não é conhecida (os
  agregados do analytics e sessões fechadas antes da migration `session/004`).
- Os demais motivos não mudam.
- O progresso da conquista SHALL mostrar o handle do jogador (admin-bff/029), com o id no tooltip; sem handle,
  SHALL mostrar o id curto, como antes.

## Critérios de Aceite

1. Sessão P2P fechada como `abandoned` mostra "Host caiu (sem heartbeat)" na tabela de sessões e no histórico do
   jogador; a dedicada continua "Servidor caiu".
2. O gráfico de motivos do analytics mostra "Servidor ou host caiu".
3. O progresso da conquista mostra o handle quando o BFF o resolve.
