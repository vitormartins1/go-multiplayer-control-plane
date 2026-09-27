# Spec: 007-login-throttle

**Serviço:** identity-service
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** lacuna G5 da revisão 2026-09 ("sem rate limit/lockout no login")

## Comportamento

- **Quando** o mesmo handle (sem diferença de maiúsculas) acumular **5 falhas** de `POST /sessions` numa janela de
  **15 min**, o identity **deve** responder `429 too_many_attempts` para esse handle até a janela vencer — sem nem
  consultar o banco nem rodar o bcrypt.
- Vale igual para handle inexistente (não revela quais handles existem).
- Login bem-sucedido zera o contador do handle.
- Estado em memória, por instância (sem dependência nova). Com várias réplicas o limite efetivo é
  5 × réplicas por janela — aceitável como primeira barreira; um limite distribuído (Redis) fica para o deploy real.
- Login Steam e anônimo não passam por aqui (a Steam já valida o ticket).
- O mapa de contadores não cresce sem limite: entradas vencidas são descartadas.

## Critérios de aceite

Unit (relógio injetado): bloqueia na 6ª tentativa, libera após a janela, sucesso zera, handle inexistente também
bloqueia, maiúsculas contam como o mesmo handle, limpeza de vencidos. Transport: 429 com o código estável.
