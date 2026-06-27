# ADR-0008: Reimplementar serviços de jogo da AWS em Go sobre compute genérico

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** todos

---

## Contexto

A AWS oferece serviços purpose-built para jogos (GameLift, GameKit, e o descontinuado GameSparks) e a Unity oferece Unity Gaming Services (Relay, Lobby, Matchmaker, Multiplay). Eles resolvem matchmaking, conquistas, identidade, leaderboards, telemetria e hosting — mas são caros e acoplam o jogo ao fornecedor. O projeto tem como tese central aprender Go construindo essa funcionalidade e comparar o resultado com a oferta gerenciada.

## Alternativas Consideradas

### Opção A: Reimplementar em Go sobre compute genérico (ECS/Lambda/EC2/EKS)
- **Prós:** controle de custo; infra reutilizável e portável; aprendizado profundo; gera o entregável de comparação (custo/perf/eficiência) da tese.
- **Contras:** reconstrói coisas que a AWS/Unity entregam prontas; mais esforço de desenvolvimento.

### Opção B: Usar os serviços gerenciados (GameLift/GameKit/Unity Gaming Services)
- **Prós:** time-to-market; menos código.
- **Contras:** custo elevado; vendor lock-in; não atende ao objetivo de aprender Go nem ao estudo comparativo; risco de descontinuação (vide GameSparks).

## Decisão

**Reimplementamos** os serviços de backend de jogo em Go, rodando em **compute genérico barato** (ver ADR-0005), e **não** usamos os serviços purpose-built pagos da AWS/Unity. Cada serviço Go forma um par de comparação com seu equivalente gerenciado, medido pela stack de observabilidade (ADR-0006). Esta é a tese do projeto.

## Consequências

### Positivas
- Custo sob controle; aprendizado; infra reutilizável para jogos próprios; entregável comparativo.

### Negativas / Trade-offs
- Mais esforço; reconstruímos funcionalidade madura existente.

### Neutras / Implicações
- Define o slate de serviços (≈ GameKit, GameLift, Game Analytics Pipeline) e a postura de não depender de serviço pago (ADR-0009, ADR-0010, ADR-0011).

## Referências
- AWS GameKit — docs.aws.amazon.com/gamekit
- AWS GameLift FlexMatch — docs.aws.amazon.com/gameliftservers
- GameSparks (descontinuado set/2022)
