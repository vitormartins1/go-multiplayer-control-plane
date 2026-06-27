# ADR-0004: Terraform como IaC, LocalStack no local e AWS em produção

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** infra, todos

---

## Contexto

A infraestrutura precisa ser versionada, reproduzível e testável localmente sem custo. Em produção roda na AWS. Cada serviço deve poder ter sua infra evoluída de forma isolada.

## Alternativas Consideradas

### Opção A: Terraform + LocalStack (local) + AWS (prod)
- **Prós:** IaC declarativa amplamente usada no mercado; LocalStack emula AWS localmente sem custo; state independente por serviço; provider único.
- **Contras:** LocalStack não tem paridade 100% com a AWS; HCL tem suas idiossincrasias.

### Opção B: AWS CDK
- **Prós:** IaC em linguagem de programação.
- **Contras:** acopla à AWS; menos mercado-agnóstico; o foco de aprendizado é Go, não TypeScript/Python de infra.

### Opção C: Pulumi
- **Prós:** IaC em linguagem de programação, multi-cloud.
- **Contras:** menos difundido em vagas; curva própria.

## Decisão

Usamos **Terraform** como IaC. Ambiente **local** aponta para **LocalStack**; **prod** para AWS real, com state no S3. Cada serviço tem **state Terraform independente**; recursos compartilhados (VPC, ECR, IAM) ficam num stack `shared` provisionado uma vez.

## Consequências

### Positivas
- Aprende-se Terraform (alto valor de mercado); dev local sem custo; isolamento de blast radius por serviço.

### Negativas / Trade-offs
- Divergências LocalStack vs AWS exigem cuidado em features de borda.

### Neutras / Implicações
- Módulos Terraform reutilizáveis por tipo de compute (ver ADR-0005).

## Referências
- Terraform AWS Provider — registry.terraform.io/providers/hashicorp/aws
- LocalStack — docs.localstack.cloud
