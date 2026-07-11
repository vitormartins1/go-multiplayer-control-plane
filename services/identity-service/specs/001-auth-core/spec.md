# Spec: 001-auth-core
**Serviço:** identity-service
**Status:** Done
**Data:** 2026-06-27

## Contexto

Núcleo de autenticação do identity-service (≈ GameKit Identity & Auth). Cobre o registro de contas de jogador, login por credenciais com emissão de um session JWT assinado, e a exposição das chaves públicas (JWKS) para que outros serviços e os game servers possam validar os tokens localmente. É a fundação de auth da qual todos os outros serviços dependem (ADR-0009).

## Comportamento

### Criar conta
- WHEN POST /accounts com handle e senha válidos -> retorna 201 com `{ id, handle, createdAt }`
- IF o handle não atender às regras de validação -> retorna 400 com **ErrInvalidHandle**
- IF a senha tiver menos de 8 caracteres -> retorna 400 com **ErrWeakPassword**
- IF já existir conta com o mesmo handle (case-insensitive) -> retorna 409 com **ErrHandleTaken**
- THE SYSTEM SHALL armazenar apenas uma representação irreversível da senha (nunca a senha em texto claro)
- THE SYSTEM SHALL garantir unicidade de handle (case-insensitive)
- THE SYSTEM SHALL nunca retornar a senha nem o hash em qualquer resposta

### Autenticar (login)
- WHEN POST /sessions com handle e senha corretos -> retorna 200 com o session JWT
- IF a senha estiver incorreta OU o handle não existir -> retorna 401 com **ErrInvalidCredentials**
- THE SYSTEM SHALL retornar resposta idêntica para senha incorreta e handle inexistente (não revelar se o handle existe)
- THE SYSTEM SHALL emitir um JWT assinado contendo, no mínimo: subject (id da conta), issued-at, expiration e um `kid` no header
- THE SYSTEM SHALL assinar o JWT com Ed25519
- THE SYSTEM SHALL fazer o token expirar

### Expor chaves públicas (JWKS)
- WHEN GET /.well-known/jwks.json -> retorna 200 com o JWKS contendo a(s) chave(s) pública(s) e seu `kid`
- THE SYSTEM SHALL nunca expor a chave privada
- THE SYSTEM SHALL permitir que qualquer consumidor (outros serviços, game servers) valide um JWT emitido usando apenas o JWKS, sem chamar o identity-service por requisição

## Contratos de Interface

### AuthService (Port de entrada)
- Register(ctx, handle, password) -> (*Account, error)
- Authenticate(ctx, handle, password) -> (token string, error)

### AccountRepository (Port de saída)
- Create(ctx, *Account) -> error
- FindByHandle(ctx, handle) -> (*Account, error)   // busca case-insensitive

### TokenIssuer (Port de saída)
- Sign(ctx, subject string) -> (token string, error)   // assina com a chave privada
- JWKS(ctx) -> (jwks, error)                            // expõe a(s) chave(s) pública(s)

## Modelo de Dados

**Account**

| Campo | Tipo | Restrições |
|---|---|---|
| ID | string (UUID) | identificador único |
| Handle | string | único case-insensitive; 3–20 chars; `[a-zA-Z0-9_]` |
| PasswordHash | string | representação irreversível da senha; nunca em claro |
| CreatedAt | timestamp | preenchido na criação |

## Regras de Validação

| Campo | Regra | Erro |
|---|---|---|
| Handle | obrigatório; 3–20 chars; `[a-zA-Z0-9_]` | ErrInvalidHandle |
| Handle | único (case-insensitive) | ErrHandleTaken |
| Password | mínimo 8 caracteres | ErrWeakPassword |
| Credenciais | handle existe e senha confere | ErrInvalidCredentials |

## Casos de Teste Obrigatórios

- [ ] Register com handle e senha válidos -> sucesso; resposta não contém senha nem hash
- [ ] Register com handle inválido (curto demais ou caractere proibido) -> ErrInvalidHandle
- [ ] Register com senha de menos de 8 chars -> ErrWeakPassword
- [ ] Register com handle duplicado case-insensitive ("Vitor" vs "vitor") -> ErrHandleTaken
- [ ] Authenticate com credenciais corretas -> retorna JWT assinado contendo sub, exp e kid
- [ ] Authenticate com senha incorreta -> ErrInvalidCredentials
- [ ] Authenticate com handle inexistente -> ErrInvalidCredentials (resposta idêntica ao caso de senha incorreta)
- [ ] Senha persistida é um hash, nunca texto em claro
- [ ] JWT emitido é verificável usando a chave pública do JWKS
- [ ] GET /.well-known/jwks.json retorna a chave pública com kid e nunca a privada

## Fora do escopo

- Refresh token e logout/revogação de sessão (incremento futuro)
- Login anônimo/guest (feature 002-anonymous-login)
- Login federado — Steam/Google (backlog)
- Account linking (backlog)
- Rotação de chaves (futuro; o `kid` já é previsto para habilitá-la sem retrabalho)
- Recuperação de senha, e-mail e verificação de e-mail
