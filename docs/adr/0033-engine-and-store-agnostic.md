# ADR-0033: Control plane agnóstico de engine e de loja (Unity e Steam como integrações de referência)

**Status:** Accepted (aprovado pelo dono em 2026-09-27)
**Data:** 2026-09-27
**Serviço(s) afetado(s):** documentação, identity-service (novos provedores), achievements-service (espelho de conquistas), data plane (SDKs por engine)
**Relaciona:** ADR-0010 (control plane × data plane; esta ADR generaliza o "Unity Dedicated Server" de lá), ADR-0031 (provedor por provedor no identity), ADR-0032 (P2P com `host_connection` opaco), `achievements/019-steam-mirror`

---

## Contexto

A ADR-0010 separou o control plane (Go) do data plane e nomeou o data plane como **Unity Dedicated Server**, porque
o primeiro jogo é Unity. Desde então, o contrato com o jogo virou gRPC e REST versionados, o ingresso (connect
token) é validado offline por chave pública (JWKS) e o `host_connection` do P2P é um identificador opaco. Nada disso
depende do Unity; o que depende é o texto da documentação e o único SDK planejado.

Com a abertura do código, o público natural inclui quem usa Godot, Unreal ou a web (Three.js). Do lado das lojas, o
identity já guarda contas externas por provedor (`external_identities(provider, external_id, account_id)`), mas só a
Steam tem verificador, e o espelho de conquistas é um worker específico da Steam.

## Alternativas Consideradas

### Opção A (escolhida): contrato agnóstico, Unity e Steam como referência; SDKs e lojas sob demanda
- A documentação passa a descrever o data plane como "o servidor ou o host do jogo, em qualquer engine", com o
  Unity como integração de referência (guia, exemplo, futuro SDK). Nada no contrato muda.
- Lojas são adaptadores: **login** é um verificador por provedor no identity (como a ADR-0031 já prevê);
  **conquistas espelhadas** passam a ser uma porta com um adaptador por loja, e hoje só existe o da Steam;
  **P2P** continua com `host_connection` opaco, prefixado pela loja (`steam:`, `eos:`).
- Novas engines e lojas entram por demanda e por custo: o que for barato e útil entra; o resto vira backlog.
- **Prós:** custa quase só documentação; não promete SDK que não existe; cada engine ou loja nova é peça nova, não
  reforma.
- **Contras:** até existir um SDK de outra engine, quem usa Godot ou Unreal integra direto pelo contrato
  (gRPC/REST), com mais trabalho.

### Opção B: SDKs para várias engines já
- **Prós:** vitrine mais forte na abertura.
- **Contras:** custo alto para manter SDKs sem usuário; o SDK Unity ainda nem foi extraído do Folclore.

### Opção C: manter o texto Unity-only
- **Prós:** zero trabalho.
- **Contras:** afasta quem não usa Unity sem motivo técnico; a documentação diria menos do que o sistema faz.

## Decisão

Opção A.

**Engines**, em ordem de custo:
1. **Unity:** referência. O SDK UPM sai do Folclore quando a camada C# estabilizar.
2. **Godot e Unreal:** integração direta pelo contrato hoje (Unreal tem gRPC em C++; no Godot o caminho mais simples
   é REST ou um cliente gRPC da comunidade). SDK só com demanda.
3. **Web (Three.js):** backlog. O navegador não fala gRPC puro nem UDP: pede uma ponte gRPC-web/Connect ou REST para
   as chamadas, e WebSocket/WebRTC no lugar do UDP. O P2P pela Steam não se aplica.

**Lojas:**
1. **Steam:** referência, completa (login, conquistas, P2P).
2. **Epic (EOS):** o login é barato. O EOS Connect entrega um ID token JWT validável pelo JWKS público
   (`https://api.epicgames.dev/epic/oauth/v2/.well-known/jwks.json`, com `iss`, `aud` e `exp`), o mesmo esquema que
   o `shared/auth` já usa. Entra como provedor `epic` numa spec própria. O espelho de conquistas depende de
   credenciais de servidor confiável (client policy "trusted server") e da ingestão de stats: fica para confirmar
   na documentação antes de virar spec.
3. **GOG:** sob demanda; as APIs de servidor para validar login e marcar conquista precisam ser pesquisadas.

## Consequências

**Positivas:**
- README, ARCHITECTURE, PRODUCT-OVERVIEW e o guia de integração descrevem o que o sistema já é: agnóstico.
- Um caminho claro para cada engine e loja nova, com o custo de cada uma registrado.

**Negativas / Trade-offs:**
- O espelho de conquistas precisa virar porta com adaptador quando a segunda loja entrar (refactor pequeno,
  adiado até lá).
- Até existir SDK, integrar outra engine exige ler o contrato.

**Neutras:**
- A ADR-0010 continua válida na essência (Go não faz realtime); só o nome do data plane deixa de ser exclusivo do
  Unity. O README de ADRs registra que a 0033 generaliza a 0010.
