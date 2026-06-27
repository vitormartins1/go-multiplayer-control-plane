# SDD-GUIDE.md — Spec-Driven Development no game-backend

## O que é SDD

Spec-Driven Development é a metodologia onde a **spec é a fonte de verdade**, não o código.
O agente de IA (Claude Code, Cursor, etc.) implementa a partir da spec, não de instruções vagas.

```
SPEC → PLAN → CODE → TEST → (repeat se spec mudar)
```

**Princípio central:** o código é o detalhe de implementação da spec. Quando o requisito muda, você edita a spec — e regenera o código que precisa mudar.

## Por que usar SDD aqui

- O agente sempre sabe o comportamento esperado antes de escrever código
- Testes são derivados diretamente dos casos listados na spec
- Drift entre intenção e implementação é detectado na review da spec, não em produção
- Múltiplos serviços mantêm coerência porque todos partem do mesmo formato

## Fluxo de trabalho

### 1. Escrever a spec (você + Claude em modo planejamento)

Antes de qualquer código:
```
"Claude, me ajude a escrever a spec do achievements-service. 
Quero que ele gerencie conquistas de jogadores, 
com desbloqueio automático por evento e listagem por jogador."
```

Claude faz perguntas, você responde, a spec é escrita em `specs/achievements-service.spec.md`.

### 2. Review da spec

Leia a spec gerada. Valide:
- Os comportamentos descritos são o que você quer?
- As regras de negócio estão corretas?
- Os casos de erro fazem sentido?

Edite até estar satisfeito. **Não avance sem validar a spec.**

### 3. Implementar a partir da spec

```
"Implemente o achievements-service seguindo a spec em 
services/achievements-service/specs/achievements-service.spec.md.
Siga a arquitetura hexagonal descrita no CLAUDE.md."
```

### 4. Gerar os testes

```
"Agora gere os unit tests para o achievements service.
Cubra todos os casos listados em 'Casos de Teste Obrigatórios' da spec."
```

### 5. Mudança de requisito

Se o requisito mudar, **edite a spec primeiro**:
```
"Adicione na spec que uma conquista pode ser revertida por um admin.
Depois atualize o código para refletir essa mudança."
```

Nunca mude o código diretamente e atualize a spec depois — a spec lidera.

---

## Formato de Spec (padrão do projeto)

```markdown
# Spec: <nome>-service

## Contexto
[Uma ou duas frases descrevendo o propósito do serviço.]

## Comportamento

### <Nome do caso de uso>
- WHEN <condição> → <resultado esperado>
- IF <condição inválida> → retorna <erro específico>
- THE SYSTEM SHALL <invariante sempre verdadeiro>

[Repita para cada caso de uso]

## Contratos de Interface

### <NomeRepository> (Port de saída)
- Metodo(ctx, param) → (retorno, error)

### <NomeService> (Port de entrada)
- Metodo(ctx, param) → (retorno, error)

## Modelo de Dados
[Campos do tipo principal com tipos e restrições]

## Regras de Validação
[Lista de validações com mensagens de erro esperadas]

## Casos de Teste Obrigatórios
- [ ] <Cenário feliz 1>
- [ ] <Cenário feliz 2>
- [ ] <Cenário de erro 1>
- [ ] <Cenário de erro 2>
- [ ] <Edge case>

## Não está no escopo
[O que este serviço explicitamente NÃO faz — evita scope creep]
```

---

## Formato EARS para regras

EARS (Easy Approach to Requirements Syntax) elimina ambiguidade:

| Padrão | Quando usar | Exemplo |
|---|---|---|
| `WHEN X → Y` | Evento que dispara comportamento | `WHEN jogador completa missão → sistema SHALL desbloquear conquista` |
| `IF X → Y` | Condição de validação | `IF nome estiver vazio → retorna ErrInvalidName` |
| `THE SYSTEM SHALL X` | Invariante sempre verdadeiro | `THE SYSTEM SHALL rejeitar nomes com mais de 50 chars` |
| `WHILE X, Y` | Comportamento contínuo | `WHILE jogador está na fila, sistema SHALL notificar status a cada 30s` |

---

## Exemplo completo — Player Service Spec

```markdown
# Spec: player-service

## Contexto
Microsserviço responsável pelo ciclo de vida de perfis de jogadores.
Gerencia criação, leitura e atualização de dados de jogadores.

## Comportamento

### Criar jogador
- WHEN POST /players com nome válido → retorna 201 + player criado com rating 1000
- IF nome estiver vazio → retorna 400 + ErrInvalidName
- IF nome tiver menos de 2 caracteres → retorna 400 + ErrNameTooShort
- IF nome tiver mais de 50 caracteres → retorna 400 + ErrNameTooLong
- IF nome já existir (case-insensitive) → retorna 409 + ErrDuplicateName
- THE SYSTEM SHALL atribuir ID único (UUID v4) a cada jogador
- THE SYSTEM SHALL atribuir rating inicial de 1000

### Buscar jogador
- WHEN GET /players/:id com ID existente → retorna 200 + player
- IF ID não existir → retorna 404 + ErrNotFound
- IF ID tiver formato inválido (não UUID) → retorna 400 + ErrInvalidID

### Atualizar progresso
- WHEN PATCH /players/:id com campos válidos → retorna 200 + player atualizado
- IF ID não existir → retorna 404 + ErrNotFound
- THE SYSTEM SHALL ignorar campos não permitidos na atualização

## Contratos de Interface

### PlayerRepository (Port de saída)
- FindByID(ctx context.Context, id string) → (*Player, error)
- FindByName(ctx context.Context, name string) → (*Player, error)
- Save(ctx context.Context, p *Player) → error
- Update(ctx context.Context, p *Player) → error

### PlayerService (Port de entrada)
- CreatePlayer(ctx context.Context, name string) → (*Player, error)
- GetPlayer(ctx context.Context, id string) → (*Player, error)
- UpdatePlayer(ctx context.Context, id string, updates PlayerUpdates) → (*Player, error)

## Modelo de Dados

```go
type Player struct {
    ID        string    // UUID v4
    Name      string    // 2–50 chars, único (case-insensitive)
    Rating    int       // default 1000, range 0–9999
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

## Regras de Validação

| Campo | Regra | Erro |
|---|---|---|
| name | len >= 2 | ErrNameTooShort |
| name | len <= 50 | ErrNameTooLong |
| name | não vazio | ErrInvalidName |
| name | único (case-insensitive) | ErrDuplicateName |
| id | UUID v4 válido | ErrInvalidID |

## Casos de Teste Obrigatórios

- [ ] CreatePlayer com nome válido → player criado com rating 1000
- [ ] CreatePlayer com nome vazio → ErrInvalidName
- [ ] CreatePlayer com nome de 1 char → ErrNameTooShort
- [ ] CreatePlayer com nome de 51 chars → ErrNameTooLong
- [ ] CreatePlayer com nome duplicado → ErrDuplicateName
- [ ] CreatePlayer com nome duplicado case-insensitive → ErrDuplicateName
- [ ] GetPlayer com ID existente → player correto
- [ ] GetPlayer com ID inexistente → ErrNotFound
- [ ] GetPlayer com ID não-UUID → ErrInvalidID

## Não está no escopo

- Autenticação/autorização (responsabilidade do API Gateway)
- Histórico de partidas (responsabilidade do matchmaking-service)
- Sistema de conquistas (responsabilidade do achievements-service)
```

---

## Dicas para prompts SDD eficazes

**Para criar uma spec:**
```
"Me ajude a escrever a spec do analytics-service no formato SDD do projeto 
(docs/sdd/SDD-GUIDE.md). O serviço recebe eventos de gameplay via POST /events, 
processa de forma assíncrona com workers, e persiste no DynamoDB."
```

**Para implementar a partir de uma spec:**
```
"Implemente o analytics-service seguindo exatamente a spec em 
services/analytics-service/specs/analytics-service.spec.md.
Use a arquitetura hexagonal do CLAUDE.md. 
Não implemente nada que não esteja na spec."
```

**Para atualizar após mudança de requisito:**
```
"A spec do player-service mudou: adicionamos soft delete. 
A spec atualizada está em services/player-service/specs/player-service.spec.md.
Atualize o código para refletir a mudança e adicione os novos casos de teste."
```

**Para revisar alinhamento spec↔código:**
```
"Revise o achievements-service e verifique se o código implementa 
todos os casos descritos na spec. Liste qualquer divergência encontrada."
```
