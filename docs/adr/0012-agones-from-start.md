# ADR-0012: Agones como orquestrador de game servers desde o início (homelab k3s / EKS sob demanda)

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** session-service, data plane (Unity), infra
**Refina:** ADR-0011

---

## Contexto

O ADR-0011 posicionou Agones/EKS como passo de escala futuro, com um pool self-managed em Go como passo intermediário. O autor prefere adotar Agones **desde o início**, por dois motivos: ter uma solução robusta e pronta para escala, e aprender Kubernetes/Agones (metas de carreira). Restrição: FinOps (~$40/mês) — o control plane do EKS sozinho custa ~$73/mês, o que estouraria o budget. O autor tem um Raspberry Pi 5 16GB (ARM64) ocioso para usar como homelab.

Pesquisa confirmou a viabilidade: k3s roda muito bem em Pi 5; o Unity 6 suporta Dedicated Server em Linux ARM64 (headless); Agones em k3s/ARM é combinação conhecida.

## Alternativas Consideradas

### Opção A: Pool self-managed em Go primeiro, Agones depois (ADR-0011 original)
- **Prós:** simples de começar, barato; ensina concorrência em Go.
- **Contras:** reimplementa o que o Agones já faz; migração futura; menos robusto.

### Opção B: Agones desde o início, dev em homelab k3s + EKS sob demanda (escolhida)
- **Prós:** robusto/pronto para escala; aprende k8s/Agones cedo (career plus); session-service nasce como cliente fino do Agones; sem migração; aproveita hardware ocioso.
- **Contras:** curva de k8s/Agones; EKS gerenciado custa — mitigado rodando k3s no homelab (grátis) e EKS efêmero só para testes de nuvem.

### Opção C: Agones com EKS sempre ligado
- **Contras:** estoura o budget (~$73+/mês só o control plane).

## Decisão

Adotamos **Agones** como orquestrador de game servers desde o início (refina ADR-0011). A **implementação** ocorre na fase de game server do roadmap (uma das últimas) — não bloqueia as features anteriores, que não tocam game servers. Ambientes:

- **Dev/homelab:** **k3s** no Raspberry Pi 5 16GB (ARM64). Grátis.
- **Cloud (sob demanda):** **EKS** efêmero (sobe/destrói via Terraform) só para testes de nuvem; nunca sempre ligado.

O `session-service` nasce como **cliente da Allocation API do Agones**; não construímos fleet manager em Go. O Unity Dedicated Server é buildado para **Linux ARM64** (Unity 6) e integra o **Agones SDK (C#)**.

## Consequências

### Positivas
- Arquitetura robusta e pronta para escala; aprendizado de k8s/Agones cedo; sem migração futura; session-service enxuto; uso do hardware ocioso.

### Negativas / Trade-offs
- Curva de Kubernetes/Agones.
- O claim atômico Ready→Allocated deixa de ser código de produção do session-service (Agones faz). O exercício de concorrência em Go migra para o matchmaking (pool de tickets) e permanece como estudo em `GO-LEARNINGS.md` §01.
- EKS exige disciplina de uso sob demanda; verificar na implementação as imagens ARM64 do Agones e a versão do Unity 6 com Dedicated Server ARM64.

### Neutras / Implicações
- Módulo Terraform de EKS + instalação do Agones; manifests de `Fleet`.
- Setup de homelab k3s documentado à parte (ver backlog).
- O game server só é exercitado na fase de session/realtime; um sandbox local de Agones pode ser montado antes como trilha de aprendizado.

## Referências
- Agones — agones.dev
- k3s — k3s.io
- Unity 6 Dedicated Server (Linux ARM64) — docs.unity3d.com
