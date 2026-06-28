# Spec: 002-anonymous-login
**Serviço:** identity-service
**Status:** Approved
**Data:** 2026-06-27

## Contexto

Permite que jogadores sem conta obtenham uma sessão para jogar/testar (login guest/anônimo). Reutiliza a emissão de JWT e o JWKS do 001-auth-core. Um guest é uma identidade **efêmera**: sem credenciais e sem persistência. Útil para integrar e testar o jogo sem exigir cadastro.

## Comportamento

### Login anônimo
- WHEN POST /sessions/anonymous -> retorna 200 com um session JWT para uma identidade guest recém-criada
- THE SYSTEM SHALL gerar um subject único para o guest (ex: `guest-<uuid>`)
- THE SYSTEM SHALL incluir no token um claim que o marca como anônimo (`anon: true`), para que serviços downstream distingam guest de jogador registrado
- THE SYSTEM SHALL assinar o token com a MESMA chave/JWKS do 001 (verificável da mesma forma, sem caminho especial)
- THE SYSTEM SHALL fazer o token expirar (como uma sessão normal)
- THE SYSTEM SHALL NOT persistir o guest (sem linha em `accounts`) — a identidade existe apenas no token

## Contratos de Interface

### AuthService (Port de entrada) — estende o 001
- LoginAnonymous(ctx) -> (token string, error)

### TokenIssuer (Port de saída) — reusado do 001
- Sign(ctx, subject string, claims ...) -> (token string, error)   // deve carregar o claim `anon`

> Nota de design: isto implica que o `TokenIssuer.Sign` do 001 seja projetado para aceitar claims extras. Refinamento a aplicar no plan/tasks do 001 para manter consistência.

## Modelo de Dados

Nenhum. O guest é stateless — não há entidade persistida nesta feature.

## Regras de Validação

Nenhuma entrada de usuário (endpoint sem corpo de requisição).

## Casos de Teste Obrigatórios

- [ ] POST /sessions/anonymous -> 200 com JWT contendo `sub` no formato `guest-...` e claim `anon: true`
- [ ] O JWT anônimo é verificável com a MESMA chave pública do JWKS (001)
- [ ] O token anônimo expira
- [ ] Nenhuma conta é criada/persistida no fluxo anônimo
- [ ] Dois logins anônimos geram subjects (guests) distintos

## Fora do escopo

- Continuidade do guest entre sessões / re-reivindicar o mesmo guest (precisaria persistir — backlog)
- Account linking: converter guest em conta registrada (backlog)
- Refresh token / logout (futuro, como no 001)
