# Prompt — revisão de produto e de código do control plane (sessão autônoma)

> Para colar (ou referenciar) no início de uma sessão do Claude Code, inclusive na nuvem (claude.ai/code).
> Mensagem mínima para abrir a sessão: *"Leia `docs/prompts/PROMPT-REVISAO-PRODUTO.md` e siga-o."*

<contexto_do_dono>
Sou o Vitor, dono deste repositório. Comecei este backend para estudar Go e ele virou um produto de verdade: um
control plane de multiplayer self-hosted (identity, game-registry, matchmaking, session com Agones, achievements,
analytics, leaderboard, cloud-save, player-data, admin-bff e um console admin em React), organizado por
arquétipo de multiplayer. Depois do MVP passei a trabalhar no jogo que o consome — o **Folclore Souls**, um
Souls-like em Unity — e fiquei quase um mês sem mexer aqui.

Quero evoluir este projeto com três objetivos, nesta ordem:
1. **Servir bem o Folclore Souls** (arquétipo `host_world_dropin`): funcionar de verdade para um jogo estilo
   FromSoftware — invasões, co-op por sinais, eventos por participante, operação barata.
2. **Ser um produto genérico bom** para outros jogos, mas genérico a partir de casos reais, não de especulação.
   Aprendi que genérico demais sem caso real vira código que ninguém usa.
3. **Ser uma vitrine do meu Go e da minha engenharia**: algo que eu apresento a outras pessoas (e talvez
   ofereça como SaaS um dia) e que mostra código Go idiomático, testado, observável e bem documentado.

Sinto que "viajei" em algumas partes (dashboards, recursos de console que ninguém usa). Não sei exatamente o que
melhorar nem o que cortar — é isso que peço a você, com autonomia.
</contexto_do_dono>

<fontes>
Leia antes de concluir qualquer coisa (e cite `arquivo:linha` quando afirmar algo sobre o código):
- `CLAUDE.md`, `AGENTS.md`, `docs/sdd/SDD-GUIDE.md`, `docs/GIT-WORKFLOW.md` — as regras da casa. Siga-as.
- `docs/PRODUCT-OVERVIEW.md`, `docs/ARCHITECTURE.md`, `docs/FEATURE-MAP.md`, `docs/ROADMAP.md`, `docs/NEXT-STEPS.md`
  (este está desatualizado desde 2026-07-13 — o `git log` é a verdade do que veio depois).
- `docs/adr/` (decisões, inclusive as `Proposed` não implementadas, como a ADR-0026) e `docs/game-models/`
  (fichas por gênero, contratos por arquétipo, follow-ups do `host_world_dropin`).
- **`docs/review/SINAIS-DO-FOLCLORE-SOULS.md`** — o que o jogo sabe e pede a este backend. O repositório do jogo
  não está visível aqui; esse documento é a ponte. Confirme no código deste repo antes de agir sobre ele.
- O código: `services/*`, `shared/`, `infra/`, os `specs/` de cada serviço e os testes.
</fontes>

<ambiente>
Você pode estar numa VM Linux na nuvem, sem o jogo, sem Unity, sem o cluster Agones (k3d/Raspberry Pi) e sem o
Docker Desktop do dono. Você tem Go, Docker, Postgres, Node e acesso à rede para dependências. Então:
- **Dá para fazer**: ler e mudar código, rodar `go build` e `go test ./... -race` (inclusive os de integração
  com testcontainers, se o Docker da VM permitir), subir partes do `docker compose` para validar ao vivo com
  `curl`/`grpcurl`, mexer no admin-ui e rodar os testes dele.
- **Não dá**: validar com o jogo rodando, alocar no Agones real, abrir o navegador com o dono olhando. O que
  precisar disso vira item registrado para o dono validar localmente, não "feito".
- O `go.mod` pede Go 1.26; se a VM tiver um Go mais antigo, use o toolchain automático (`GOTOOLCHAIN=auto`).
</ambiente>

<missao>
Trabalhe em duas fases. Não pule a primeira.

**Fase 1 — Revisão (leitura e medição, sem mudar código de produto).**
Descubra o estado REAL: rode os testes, conte o que existe, verifique o que cada peça faz e quem usa. Avalie
pelas três lentes do dono (Folclore Souls · produto genérico · vitrine de Go). Entregue
`docs/review/REVISAO-2026-09.md` com:
1. **Estado verificado** — o que existe e funciona (com evidência: teste, comando, `arquivo:linha`), e onde a
   documentação diverge do código.
2. **O que cortar ou simplificar** — recursos que custam manutenção e não servem a nenhum jogo real nem à
   vitrine. Diga o custo de manter × o valor, e se a recomendação é remover, congelar ou simplificar.
3. **O que falta** — lacunas para o Folclore Souls (partindo dos sinais), para um produto genérico sério
   (segurança, idempotência, falhas parciais, operação, custo) e para a vitrine (qualidade do Go, testes,
   observabilidade, README de primeira impressão, demo reproduzível).
4. **Riscos e dívida técnica**, com severidade.
5. **Roadmap priorizado** — itens pequenos e verificáveis, cada um com impacto, esforço, lente atendida e o
   critério de "feito". Ordene por valor/esforço. Diga quais você executa na Fase 2.
6. **Decisões que só o dono pode tomar** — poucas, cada uma com as opções, a sua recomendação e o porquê.

**Fase 2 — Execução autônoma.**
Execute, em ordem, os itens do roadmap que NÃO dependem de decisão do dono, um por vez, cada um pelo fluxo da
casa (SDD: `spec.md`/`plan.md`/`tasks.md` quando o item for uma feature; commit direto quando for doc ou
correção pequena). Cada item termina com testes verdes e a documentação atualizada (inclusive o `NEXT-STEPS.md`).
Continue até esgotar os itens executáveis ou o contexto; se o contexto acabar, a nota de progresso (abaixo)
garante que a próxima sessão continue de onde parou.
</missao>

<como_trabalhar>
- **Evidência antes de opinião.** Não afirme nada sobre código que você não abriu. Recurso "sem uso" precisa de
  prova (nenhum chamador, nenhuma rota consumida pelo admin-bff/admin-ui, nenhum teste de integração que o
  exercite). Se a medição não for possível, diga que é hipótese.
- **Simplicidade.** Prefira remover e simplificar a acrescentar. Não crie abstração para um caso único. Não
  generalize sem um caso real que peça. Mudança pequena e testada vale mais que refatoração grande.
- **Testes são a régua.** `go test ./... -race` verde antes e depois de cada item. Não afrouxe nem apague teste
  para ficar verde; se um teste estiver errado, prove e corrija o teste com a justificativa no commit.
- **Contrato com o jogo é sagrado.** O `host_world_dropin-dataplane-contract.md` e os `.proto` são usados pelo
  jogo em produção de desenvolvimento. Mudança que quebra o contrato só com decisão do dono; mudança compatível
  (campo novo opcional) documenta o impacto no jogo.
- **Progresso em arquivo.** Mantenha `docs/review/PROGRESSO.md` atualizado a cada item: o que foi feito (commit),
  o que está em andamento, o próximo passo e o que ficou para o dono validar. É o que sobrevive ao fim do
  contexto e a uma sessão nova.
- **Git.** Trabalhe numa branch própria (`review/produto-2026-09`, e sub-branches por feature se o fluxo da casa
  pedir), Conventional Commits com escopo de serviço, um commit por entrega. **Não faça push na `main`**: ao
  final, faça push da sua branch — é assim que o trabalho sai da nuvem para o dono revisar.
- **Idioma.** Escreva docs, commits e respostas em português, com acentuação correta.
</como_trabalhar>

<quando_parar_e_perguntar>
Não pare para pedir permissão para ler, medir, testar ou executar itens do roadmap que não dependem do dono.
Pare e pergunte (ou registre em "Decisões que só o dono pode tomar" e siga para o próximo item) quando:
- a mudança remove um serviço inteiro ou uma rota que o jogo ou o console usam;
- a mudança quebra o contrato com o jogo ou muda um `.proto` de forma incompatível;
- a decisão é de direção de produto (cobrar, abrir código, trocar stack, mudar a tese do projeto);
- precisa de credencial, conta paga ou recurso que a VM não tem.
</quando_parar_e_perguntar>

<resposta_final>
Ao terminar (ou quando o contexto estiver acabando), responda em português, curto:
- o link/nome da branch com o push feito e os commits principais;
- o resumo da revisão em 5-10 linhas (o que cortar, o que falta, o que foi feito);
- as decisões pendentes para o dono, cada uma com a sua recomendação;
- o que o dono precisa validar localmente (com o jogo ou o Agones) e como.
</resposta_final>
