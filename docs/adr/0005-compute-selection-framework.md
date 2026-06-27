# ADR-0005: Seleção de compute por serviço (decisão diferida)

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** todos, infra

---

## Contexto

Os serviços têm perfis de carga diferentes (CRUD stateless, worker always-on, ranking, streaming stateful, game server realtime). Forçar um único tipo de compute para todos seria subótimo em custo e didática. O autor desenvolve os serviços um por um e quer decidir a infra de cada um na hora de construí-lo.

## Alternativas Consideradas

### Opção A: Framework de seleção, decisão diferida por serviço
- **Prós:** cada serviço usa o compute que combina com seu formato; aprende-se múltiplos modelos (Lambda, Fargate, EC2, EKS); controle de custo por perfil.
- **Contras:** múltiplas stacks de infra e módulos Terraform; mais superfície operacional.

### Opção B: Padronizar tudo em ECS Fargate
- **Prós:** uniformidade; um módulo de infra.
- **Contras:** caro/subótimo para CRUD bursty (Lambda escala a zero); não exercita outros modelos.

## Decisão

**Não** padronizamos compute. Definimos um **menu** — Lambda, ECS Fargate, ECS-on-EC2, EC2, EKS — e a escolha é feita **por serviço, no `plan.md` da feature**, guiada pelo formato do serviço. Critérios: CRUD stateless/bursty → Lambda; worker always-on → ECS/EC2; streaming/stateful → ECS/EKS; game server realtime (UDP, portas dinâmicas) → EC2/ECS-on-EC2 → Agones/EKS (nunca Fargate). EKS só com aplicação concreta que o justifique.

Esta é uma **meta-decisão**: decidimos *como* escolher compute e *adiar* a escolha vinculante de cada serviço. Nenhum outro ADR deve fixar o compute de um serviço — isso é prerrogativa do `plan.md`, regido por este ADR.

## Consequências

### Positivas
- Custo e didática otimizados por serviço; flexibilidade.

### Negativas / Trade-offs
- Mais módulos Terraform e padrões operacionais (um por tipo de compute).

### Neutras / Implicações
- A escolha de banco de dados de cada serviço (gerenciado vs auto-provisionado) segue a mesma lógica diferida.

## Referências
- ADR-0008 (compute genérico vs serviços-de-jogo AWS)
- AWS Lambda / ECS / EKS — docs.aws.amazon.com
