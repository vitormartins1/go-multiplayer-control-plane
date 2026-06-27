# Aprendizados de Go — game-backend

> Caderno vivo das coisas boas de Go que fui aprendendo ao construir este projeto.
> Cada entrada: o conceito, **por que importa**, um exemplo comentado e **onde uso no projeto**.
> Atualizado de forma incremental conforme uso algo novo.

## Como usar

- Uma entrada por conceito/padrão. Adicione ao final, numerando.
- Prefira exemplo curto e comentado a teoria longa.
- Ligue ao serviço/lugar real onde o padrão aparece.

### Template de entrada

```markdown
## NN — Título do conceito

**O que é / por que importa:** ...
**Onde no projeto:** <serviço/arquivo>

​```go
// exemplo comentado
​```

**Pegadinha / nuance:** ...
```

---

## 01 — Concorrência: alocação atômica com `sync.Mutex`

**O que é / por que importa:** quando várias goroutines disputam o mesmo recurso, "ler o estado e agir sobre ele" precisa ser **uma operação indivisível**. Em Go, o jeito mais direto é proteger a seção crítica com `sync.Mutex`. É o tipo de problema em que Go é confortável e expressivo.

**Onde no projeto:** `session-service` — alocação de game server (escolher UM server `Ready` e reivindicá-lo sem que duas partidas peguem o mesmo). Ver `docs/ARCHITECTURE.md` §7.

```go
package registry

import (
	"errors"
	"sync"
)

type State int

const (
	StateReady State = iota
	StateAllocated
	StateDown
)

type GameServer struct {
	ID     string
	Addr   string // ip:porta
	Region string
	State  State
}

var ErrNoServerAvailable = errors.New("session: nenhum game server disponível")

// Registry guarda os game servers e permite alocação concorrente segura.
type Registry struct {
	mu      sync.Mutex
	servers map[string]*GameServer
}

// Allocate reivindica UM server Ready da região, de forma atômica.
//
// Sem o lock, duas goroutines (duas partidas fechando ao mesmo tempo)
// poderiam ler o mesmo server "Ready" e ambas marcá-lo Allocated —
// resultado: dois matches no mesmo server. O mutex garante que o
// "ler estado + reivindicar" aconteça como uma operação indivisível.
func (r *Registry) Allocate(region string) (*GameServer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, gs := range r.servers {
		if gs.State == StateReady && gs.Region == region {
			gs.State = StateAllocated // claim dentro da seção crítica
			return gs, nil
		}
	}
	return nil, ErrNoServerAvailable
}
```

**Pegadinha / nuance:** o `sync.Mutex` resolve a corrida **dentro de um processo**. Se rodarem várias instâncias do `session-service` (escala horizontal), cada uma tem seu próprio lock em memória — o mutex não basta. Aí o claim atômico precisa migrar para o **store compartilhado**:

- DynamoDB: `UpdateItem` condicional — `SET state = :allocated` com `ConditionExpression: state = :ready`. Se a condição falhar, outra instância já pegou.
- Postgres: `UPDATE servers SET state='allocated' WHERE id=$1 AND state='ready' RETURNING *` — se não voltar linha, já estava ocupado.

Ou seja: o conceito (ler-e-reivindicar atômico) é o mesmo; só muda **onde** a atomicidade é garantida — no mutex local ou no banco distribuído.
