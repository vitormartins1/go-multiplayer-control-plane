# Política de segurança

## Versões suportadas

O projeto ainda não tem releases versionadas: correções de segurança entram na `main`. O contrato com o jogo tem
versão própria ([`docs/contracts/CHANGELOG.md`](docs/contracts/CHANGELOG.md)), e uma correção que exija mudar o
contrato é anunciada lá.

## Como reportar uma vulnerabilidade

**Não abra uma issue pública.** Use o reporte privado do GitHub: aba **Security** do repositório →
**Report a vulnerability**. Descreva o problema, como reproduzir e o impacto que você enxerga.

Você recebe uma primeira resposta em até 7 dias. Depois da correção, o crédito é seu no aviso de segurança, se
quiser.

## O que está no escopo

Os serviços em Go deste repositório, o console (`services/admin-ui` e `admin-bff`) e a infraestrutura de exemplo
(`infra/`). Algumas áreas merecem atenção especial:

- **Tokens**: o JWT do jogador (identity), o connect token Ed25519 (session) e o `world_token` de um mundo P2P.
- **Autenticação do gRPC do session** (ADR-0030): quem pode chamar cada RPC.
- **Login Steam** e o espelho de conquistas: a chave de publisher da Steam nunca pode vazar em log, erro ou resposta.
- **Login Epic**: o ID token do EOS Connect é validado pelo JWKS da Epic (assinatura, `iss`, `aud`, `exp`, `iat`).

## Limitações conhecidas (não são vulnerabilidades)

Estão documentadas e são escolhas atuais do projeto:

- O **console não tem login**. Ele não deve ser exposto na internet; rode-o atrás de VPN ou de um proxy com
  autenticação.
- O gRPC do session sobe em modo `warn` (registra, mas aceita, chamadas sem credencial) até os clientes migrarem;
  em produção, use `SESSION_AUTH_MODE=enforce`.
- As chaves em `infra/docker/secrets/` são geradas localmente por `gen-secrets.sh` e servem só para
  desenvolvimento.
