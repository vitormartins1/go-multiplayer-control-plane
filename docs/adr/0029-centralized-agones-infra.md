# ADR-0029: Infra Agones centralizada no control plane

**Status:** Proposed
**Data:** 2026-07-10
**Serviço(s) afetado(s):** control plane (`infra/`), todos os `unity-<archetype>-data-plane`
**Relaciona:** ADR-0010 (control plane orquestra), ADR-0012 (Agones desde o início), ADR-0026 (Fleet por jogo), `REPO-TOPOLOGY.md`

---

## Contexto

Hoje toda a infra Agones (bootstrap do cluster k3s/k3d, instalação do Agones via Helm, kubeconfig, monitoramento Grafana, **e** o `fleet.yaml` por arquétipo) vive dentro do `unity-queue-arena-data-plane` (`tools/pi5/`, `tools/k3d/`, `deploy/agones/`) — não porque seja conceitualmente daquele arquétipo, mas porque foi o primeiro (e único até agora) data plane real. Nada disso é específico do `queue_arena`: é infra do **cluster**, que qualquer jogo vai usar. Prova concreta: o `host_world_dropin` (Fase 5 pendente) hoje teria que copiar `tools/pi5/`/`tools/k3d/` inteiros pro próprio repo, duplicando bootstrap de k3s, instalação de Agones, kubeconfig, monitoramento — só pra ganhar uma Fleet nova.

Transporte de imagem hoje é manual e sem registry: `docker save | ssh | k3s ctr images import` (Pi 5, `tools/pi5/push-image.sh`) ou `k3d image import` (local, `tools/k3d/import-image.sh`). Funciona para um único desenvolvedor com acesso direto às duas máquinas, mas não escala para múltiplos devs, onboarding de terceiros, nem um cenário de open-source/handoff — perguntas levantadas pelo usuário ao revisar este ADR.

A pergunta que motivou este ADR (do usuário, ao revisar o rollout do ADR-0026): dado que a infra Agones está hoje centralizada "por acidente" no primeiro data plane, não faria mais sentido ela morar no control plane — que já é quem orquestra (ADR-0010) e, desde o ADR-0026, quem sabe qual Fleet cada jogo usa?

## Alternativas Consideradas

### Opção A: Status quo — infra Agones dentro de cada data plane
- **Prós:** zero trabalho; cada repo é self-contido para quem só mexe naquele arquétipo.
- **Contras:** duplica bootstrap de cluster (k3s, Agones, kubeconfig, monitoramento) a cada novo arquétipo; contradiz o próprio ADR-0026 (control plane já é a fonte de verdade de qual Fleet existe, mas não é quem aplica); `host_world_dropin` já provou o problema (copiaria tudo de novo na Fase 5).

### Opção B: Centralizar bootstrap + Fleet manifests no control plane, build/push fica no data plane (escolhida)
- **Prós:** elimina a duplicação; consistente com ADR-0010/0026; um único `apply-fleet.sh <nome>` para todas as Fleets.
- **Contras:** iterar num data plane deixa de ser 100% self-contido (aplicar uma Fleet sai da pasta do jogo); aceitável hoje porque é um único desenvolvedor cuidando dos N repos.

### Opção C: Mover tudo (incluindo build/push da imagem) para o control plane
- **Contras:** build de imagem Unity (IL2CPP, targets ARM64/headless) é inerentemente acoplado ao código-fonte e às ferramentas daquele repo (Unity Editor, assemblies) — não faz sentido o control plane (Go, sem Unity) assumir isso. Descartada.

## Decisão

Adotamos a **Opção B**, em duas fatias deliberadamente separadas por horizonte de tempo.

### Fatia 1 (agora) — centralizar o que já é do control plane

- **Bootstrap de cluster + ferramentas compartilhadas** (`install-k3s.sh`, `install-agones.sh`, kubeconfig fetch/tunnel, dashboard Grafana) migram para `go-multiplayer-control-plane/infra/agones/` (irmão de `infra/terraform/`, `infra/docker/`) — hoje já é logicamente do cluster inteiro, só está fisicamente no lugar errado.
- **Fleet manifest por jogo** (`fleet.yaml`) também migra, um arquivo por Fleet: `infra/agones/fleets/<fleet-name>.yaml` (nome = o mesmo valor cadastrado via `PUT /games/{id}/fleet`, ADR-0026) — operacionaliza o que o ADR-0026 já formalizou: o control plane é quem sabe qual Fleet cada jogo usa, então também é quem aplica.
- **Fica no repo do data plane:** só o que constrói a imagem (`build-server.sh`, `import-image.sh`, `push-image.sh`) — a Fleet manifest referencia a imagem resultante por uma convenção de nome fixa (`<fleet-name>-server:latest`), sem automação ainda.
- Script `infra/agones/apply-fleet.sh <fleet-name>` único, em vez de um `apply-fleet.sh` por repo.

### Fatia 2 (futuro, gated por evidência — não implementar agora)

- `domain.Game` ganha uma referência ao **repositório/registry de imagem** do data plane daquele jogo (ex.: `imageRepository: ghcr.io/vitormartins1/unity-host-world-dropin-data-plane`), cadastrável no console — mesmo padrão do campo `Fleet` (ADR-0026).
- CI (GitHub Actions) em cada `unity-<archetype>-data-plane` builda e publica a imagem no **GHCR** (GitHub Container Registry) a cada release — grátis, já integrado ao GitHub onde os repos já vivem (`vitormartins1/*`), sem serviço novo pra manter (Artifactory teria custo/operação extra sem ganho claro aqui).
- A Fleet manifest passa a referenciar `<imageRepository>:<tag>` escolhido no console por jogo, em vez de `latest` fixo — permite trocar a imagem de uma Fleet sem editar YAML à mão.
- Isso destrava múltiplos devs, handoff pra um amigo testar, ou um caminho pra open-source — cada um só precisa de acesso ao próprio repo + a chave/token de imagem, sem tocar no cluster ou nos scripts alheios.

**Gate da Fatia 2:** não implementar até que o processo manual da Fatia 1 realmente doa (2º arquétipo real em produção, ou necessidade concreta de outro dev/ambiente) — mesmo critério já usado no M2 dos modos e no SDK Unity compartilhado (`REPO-TOPOLOGY.md`, princípio 5).

## Consequências

### Positivas
- Elimina duplicação de bootstrap por arquétipo — o `host_world_dropin` (e os próximos) ganham Fleet sem reinstalar cluster.
- Centraliza "o que sabe sobre Fleets" no mesmo lugar que já resolve o *nome* da Fleet (ADR-0026) — coerência arquitetural, não dois sistemas de verdade.
- Caminho de evolução (Fatia 2) fica desenhado e documentado, não precisa ser redescoberto sob pressão quando a dor aparecer.

### Negativas / Trade-offs
- Iterar num data plane deixa de ser 100% self-contido — aplicar uma Fleet sai da pasta do jogo e vai no control plane. Aceitável hoje (um único desenvolvedor, N repos); reavaliar se isso incomodar na prática.
- Fatia 1 ainda é manual (convenção de nome de imagem, sem registry) — só menos duplicada, não automatizada. É uma melhoria incremental, não a solução final.

### Neutras / Implicações
- Fatia 2 é intencionalmente vaga em detalhes técnicos (schema exato do campo, mecanismo de auth do GHCR, formato do token) — vira spec própria só quando o gate abrir; não faz sentido detalhar agora e arriscar ficar obsoleto.
- Nenhuma mudança de código nesta decisão — é só um registro de direção; a Fatia 1 vira uma spec própria (provavelmente um doc de infra, não um serviço) quando for implementada.

## Referências
- `unity-queue-arena-data-plane/tools/pi5/`, `tools/k3d/`, `deploy/agones/` — estado atual investigado em 2026-07-10.
- ADR-0010 — control plane como orquestrador.
- ADR-0012 — Agones desde o início.
- ADR-0026 — Fleet por jogo (o gancho que motivou esta discussão).
- `REPO-TOPOLOGY.md` princípio 5 — mesmo padrão de "adiar até o gate" já usado para o SDK Unity compartilhado.
