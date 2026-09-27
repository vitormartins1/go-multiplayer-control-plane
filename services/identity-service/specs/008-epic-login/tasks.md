# Tasks: 008-epic-login

- [x] T1 `internal/epic/verifier.go`: cache de JWKS (RSA/EC, TTL, refetch limitado, singleflight, fallback) e `Verify`
- [x] T2 testes do verificador com JWKS falso (httptest) e tokens assinados no teste: válido, assinatura trocada,
      `alg: none`, HMAC, `iss` de outro host, `aud` de outro produto, expirado, `iat` no futuro, `kid` novo depois
      de rotação, JWKS fora com e sem chave em cache
- [x] T3 `cmd/server/main.go`: provedor `epic` com `EPIC_CLIENT_ID`/`EPIC_JWKS_URL`, independente da Steam
- [x] T4 teste do service/transport: `POST /sessions/epic` cai no mesmo fluxo (a rota e o service já cobrem; conferir)
- [x] T5 validação ao vivo: identity com JWKS falso → `POST /sessions/epic` → JWT aceito pelo matchmaking/session
- [x] T6 docs: compose (envs vazias), README do identity, guia de integração, `docs/contracts/CHANGELOG.md`
- [x] T7 GO-LEARNINGS (se houver padrão novo) e resumo didático
