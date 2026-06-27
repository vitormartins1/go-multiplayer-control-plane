# ADR-0002: Hexagonal Architecture idiomática em Go em todos os serviços

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** todos

---

## Contexto

Todos os serviços precisam de uma arquitetura interna consistente, testável e que isole regra de negócio de I/O. O autor vem de C#/.NET, onde Clean Architecture/Onion é comum — mas Go é menos dogmático sobre camadas e penaliza over-abstraction (interfaces declaradas no consumidor, "accept interfaces, return structs"). Os serviços têm formatos variados (CRUD, regras ricas, ranking, pipeline assíncrono, concorrência).

## Alternativas Consideradas

### Opção A: Hexagonal (Ports & Adapters)
- **Prós:** isola o core de HTTP/banco; testes triviais via mock de interface; mapeia bem para `domain/service/repository/transport`; dobra-se bem a todos os formatos de serviço.
- **Contras:** risco de cerimônia em serviços de CRUD raso.

### Opção B: Clean / Onion (estilo .NET)
- **Prós:** familiar para quem vem de .NET.
- **Contras:** prescritivo demais; tende a virar interface pollution em Go.

### Opção C: Vertical Slice
- **Prós:** baixa abstração; pacote por feature mapeia bem em Go.
- **Contras:** menos consistência entre serviços; mistura camadas por feature.

## Decisão

Adotamos **Hexagonal uniforme** nos quatro layers `domain / service / repository / transport`, aplicado com idioma de Go. O `service/` só depende de interfaces declaradas em `domain/`; nunca importa `transport/` nem `repository/`.

Regra anti-interface-pollution: uma interface só existe quando **(a)** é mockada em teste, **(b)** tem 2+ implementações reais, ou **(c)** cruza uma fronteira de I/O. Caso contrário, struct concreta. Organização vertical-slice **dentro** de um serviço é permitida quando ele acumular muitas features.

## Consequências

### Positivas
- Testabilidade alta; consistência entre serviços; previsível para o agente de IA.
- Um único padrão exercitado contra CRUD, regras, ranking, pipeline e concorrência (didático).

### Negativas / Trade-offs
- Em serviços de CRUD raso, parte da estrutura é cerimônia — aceita pelo valor de consistência/aprendizado.

### Neutras / Implicações
- A régua de interfaces precisa ser aplicada em revisão para evitar abstração desnecessária.

## Referências
- Alistair Cockburn — Hexagonal Architecture (Ports & Adapters)
- Go Proverbs — "accept interfaces, return structs"
