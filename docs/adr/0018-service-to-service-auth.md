# ADR-0018: Autenticação serviço-a-serviço (internal API key, JWT como evolução)

**Status:** Accepted
**Data:** 2026-06-29
**Serviço(s) afetado(s):** todos (control plane interno), infra
**Relaciona:** ADR-0009 (identity/JWT do jogador), ADR-0010 (control plane interno), ADR-0016/0017 (endpoints `/admin`)

---

## Contexto

Vários caminhos hoje são **control plane interno sem auth** ("a barreira é a rede"): os `/admin/*` (identity, achievements, game-registry), o admin-bff → serviços, matchmaking → session. Adiamos a auth serviço-a-serviço repetidamente. O **fan-out do resultado** (session → achievements/leaderboard) será o primeiro **write** serviço-a-serviço, e expor um write sem nenhuma credencial não é aceitável nem em dev sério. Precisamos de um mecanismo simples, barato (ADR-0008) e evolutivo.

O identity já emite **JWT EdDSA do jogador** + JWKS; os serviços validam o token do **jogador** via `shared/auth`. Isso é auth de jogador, não de serviço.

## Alternativas Consideradas

### Opção A: Internal API key compartilhada (header), validada por middleware compartilhado
- **Prós:** mínima (env var + um middleware em `shared`); sem infra nova; cobre todos os `/admin` e os writes; upgradeável. Padrão comum de "internal token".
- **Contras:** segredo único compartilhado (não distingue o chamador); rotação manual; vazamento compromete tudo até a rotação.

### Opção B: Service JWT (client-credentials) — um emissor de tokens de serviço
- **Prós:** identidade por serviço, expiração, revogação por chave; reaproveita JWKS.
- **Contras:** precisa de emissor/fluxo de credenciais de serviço; mais peças; overkill agora.

### Opção C: mTLS entre serviços
- **Prós:** padrão forte de prod; identidade mútua.
- **Contras:** gestão de certificados pesada para local-first; cedo demais.

### Opção D: Manter sem auth (rede como barreira)
- **Contras:** write serviço-a-serviço sem credencial; não escala para deploy real. **Rejeitada.**

## Decisão

Adotamos a **Opção A** no MVP:

1. **`INTERNAL_API_KEY`** (env, por ambiente) — **uma chave compartilhada** pelos serviços do control plane (por-serviço/JWT é evolução).
2. As chamadas **serviço-a-serviço** e os endpoints **`/admin/*`** exigem o header `X-Internal-Key: <chave>`; um **middleware em `shared`** valida (ausente/errada → `401`). Os callers (admin-bff, session no fan-out, etc.) injetam o header via env.
3. **Rotas de jogador continuam com o JWT do jogador** (ADR-0009) — auth de jogador e de serviço são camadas distintas.
4. **Defesa em profundidade:** a rede interna continua barreira; a chave é a segunda camada.
5. **Evolução:** migrar para **service JWT (Opção B)** quando houver múltiplos consumidores/ambientes que exijam identidade por serviço e revogação fina; mTLS no hardening de prod.

## Consequências

### Positivas
- Fecha o buraco de auth dos `/admin` e habilita o fan-out com segurança mínima; barato; um único middleware reusável.

### Negativas / Trade-offs
- Segredo único (sem identidade por serviço); rotação manual; vazamento compromete tudo até a rotação.

### Neutras / Implicações
- `shared/auth` ganha um `InternalKeyMiddleware`; cada serviço com `/admin` passa a exigir a chave; os callers (admin-bff, session) injetam-na.
- O painel (admin-ui) chama o admin-bff (que detém a chave e a repassa) — o browser **não** vê a chave.
- Local: `INTERNAL_API_KEY` no compose (dev, valor fixo); prod via secret.
- O fan-out do resultado precisa, no achievements, de um **endpoint server-side** (registrar evento por jogador sem o JWT do jogador), protegido por esta chave.

## Referências
- ADR-0009 (JWT do jogador), ADR-0010 (control plane), ADR-0016/0017 (endpoints admin), ADR-0008 (custo/local-first)
