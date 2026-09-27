# Plan: 008-epic-login

- **D1 — Adapter novo, nada muda no service nem no transport.** `internal/epic` implementa a Port
  `domain.ExternalIdentityVerifier` (`Provider() == "epic"`), igual ao `internal/steam`. O `AuthenticateExternal`,
  a criação de conta, o vínculo em `external_identities` e os erros HTTP já existem (spec 005).
- **D2 — Cache de chaves próprio, sem mexer em `shared/`.** O `shared/auth` só lê chaves Ed25519 (as do nosso
  identity) e exige o `iss`/`aud` do control plane; o ID token do EOS vem assinado com RSA (ou EC). O verificador
  da Epic tem o próprio cache de JWKS, com o mesmo desenho do `shared/auth`: chave por `kid`, TTL, nova busca só
  para `kid` desconhecido, no máximo uma a cada 30 s, buscas simultâneas viram uma (`singleflight`) e chave em
  cache continua valendo se a Epic estiver fora.
- **D3 — Chaves e algoritmos aceitos.** JWK `RSA` (RS256/RS384/RS512) e `EC` P-256/P-384 (ES256/ES384). O
  algoritmo do token precisa bater com o tipo da chave do `kid` (o `golang-jwt` recusa chave do tipo errado);
  `none` e HMAC nunca passam (`WithValidMethods`).
- **D4 — Claims.** `aud` = `EPIC_CLIENT_ID` (`WithAudience`), `exp` obrigatório, `iat` no passado
  (`WithIssuedAt`), sem tolerância de relógio (como o resto do projeto). O `iss` é checado à mão: URL `https` com
  host exato `api.epicgames.dev` (refinamento da spec: prefixo de string aceitaria outro domínio). O `sub` é o PUID.
- **D5 — Handle.** Reusa o `externalHandle` da spec 005: `epic_` + os últimos 15 caracteres do PUID (limite de 20
  do handle). O PUID é hexadecimal de 32 caracteres; 15 hex são 60 bits, colisão desprezível, e se acontecer o
  plano B de handle aleatório da spec 005 cobre.
- **D6 — Erros.** Token vazio, grande demais (> 8 KiB), malformado ou com claims errados → `ErrInvalidTicket`
  (401). JWKS inalcançável e nenhuma chave em cache para o `kid` → `ErrProviderUnavailable` (503).
- **D7 — Configuração.** `EPIC_CLIENT_ID` liga o provedor; `EPIC_JWKS_URL` troca o endpoint (padrão
  `https://api.epicgames.dev/epic/oauth/v2/.well-known/jwks.json`; um JWKS falso nos testes e na validação ao vivo).
  `main.go` registra Steam e Epic de forma independente.
- Arquivos: `internal/epic/verifier.go` (+ `_test.go`), `cmd/server/main.go`, `infra/docker/docker-compose.yml`
  (envs vazias), README do identity, guia de integração, `docs/contracts/CHANGELOG.md`.
