# Spec: 008-epic-login

**Serviço:** identity-service
**Status:** Draft (aguardando aprovação)
**Data:** 2026-09-27
**Decisão:** ADR-0033 (Epic como segunda loja, começando pelo login) sobre a ADR-0031 (provedor por provedor)

## Contexto

O identity já aceita login pela Steam (`POST /sessions/steam`, spec 005): valida o ticket da loja, vincula o ID
externo a uma conta (`external_identities`) e emite o JWT de sempre. A Epic entra pela mesma porta. O caminho mais
barato é o **EOS Connect**: o jogo faz login no EOS com a identidade que tiver (conta Epic, Steam, console) e recebe
um **ID token** (JWT) com o `ProductUserId` (PUID) do jogador. O backend valida esse token sozinho, pela chave
pública que a Epic publica (JWKS), sem chamar API nenhuma da Epic no caminho do login. O PUID também é o
identificador que o P2P do EOS usa (`host_connection = "eos:<PUID>"`, ADR-0032), então login e P2P falam do mesmo
jogador.

## Comportamento

- `POST /sessions/epic` com `{"ticket": "<ID token do EOS Connect>", "gameId": "<opcional>"}`: mesmo corpo e mesma
  rota parametrizada da Steam.
- O token SHALL ser aceito só se:
  - a assinatura confere com uma chave do JWKS da Epic (`EPIC_JWKS_URL`, padrão
    `https://api.epicgames.dev/epic/oauth/v2/.well-known/jwks.json`), e o `alg` existe e não é `none`;
  - o `iss` começa com `https://api.epicgames.dev`;
  - o `aud` é o client ID do produto (`EPIC_CLIENT_ID`);
  - o `iat` está no passado e o `exp` no futuro (com a mesma tolerância de relógio do resto do projeto).
- WHEN o token é válido -> THE SYSTEM SHALL achar a conta vinculada ao PUID (`external_identities`, provider `epic`)
  ou, na primeira vez, criar a conta (handle `epic_<12 últimos caracteres do PUID>`, sem senha) e vincular; emitir o
  JWT de sempre com `idp: "epic"` e marcar presença no jogo se `gameId` vier (ADR-0019). Mesmo fluxo da Steam.
- Erros, iguais aos da Steam: sem `EPIC_CLIENT_ID` o provedor não é registrado → 404 `provider_not_supported`;
  token malformado, assinatura inválida, `iss`/`aud`/`exp` errados → 401 `invalid_ticket`; JWKS da Epic
  inalcançável e sem chave em cache → 503 `provider_unavailable`.
- As chaves do JWKS SHALL ficar em cache, com nova busca quando aparecer um `kid` desconhecido e limite de
  frequência, como já é feito para o JWKS do próprio identity.
- Conta criada pela Epic não tem senha: `POST /sessions` com handle e senha nunca autentica nela.

## Critérios de Aceite

1. Verificador: token válido; assinatura trocada; `alg: none`; `iss` de outro domínio; `aud` de outro produto;
   expirado; `kid` desconhecido que aparece depois de uma nova busca no JWKS (servidor JWKS falso com httptest e
   tokens assinados no teste).
2. Service: primeira vez cria e vincula; segunda vez reutiliza; handle ocupado tenta um alternativo (mocks, como na
   spec 005).
3. Transport: os códigos HTTP acima; `POST /sessions/steam` continua igual.
4. Ao vivo, com um JWKS falso: login pela Epic → JWT do identity aceito pelo matchmaking e pelo session.
5. Documentação: `.env.example`/compose com as variáveis, guia de integração (como o jogo pega o ID token no EOS
   Connect) e o changelog do contrato (rota nova, aditiva).

## Fora de escopo

- Espelho de conquistas na Epic: depende de credenciais de servidor confiável e da ingestão de stats do EOS; a API
  precisa ser confirmada na documentação antes de virar spec (ADR-0033).
- Login pelo Epic Account Services (EAS) com o `EpicAccountId`: o Connect cobre mais plataformas e é o ID do P2P.
- Vincular a mesma pessoa entre lojas (Steam e Epic na mesma conta): cada loja cria a sua conta, como hoje.

## Notas para o plan

- Os critérios de validação vêm da referência da Connect Interface do EOS; conferir de novo na documentação
  oficial ao escrever o `plan.md` (o proxy da sessão que escreveu esta spec bloqueava o site da Epic).
- O `shared/auth` já sabe buscar e cachear JWKS, mas valida `iss`/`aud` do próprio control plane. Reaproveitar a
  busca sem mudar o comportamento do `shared/` é decisão do plan (mudança em `shared/` pede consulta ao dono).
