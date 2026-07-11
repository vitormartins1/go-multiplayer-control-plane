# Spec 019 — Expand da sessão por arquétipo (layout do console + occupants)

**Serviço:** admin-ui (redesign do painel) + session-service/admin-bff (campo aditivo `occupants`)
**Status:** Done (2026-07-08; pedido do usuário: repensar UX/layout do expand da sessão com um layout por arquétipo)
**Fatia:** sucessora direta da spec 018 (que trouxe os dados); esta reorganiza a apresentação.

## Contexto

A 018 enriqueceu o expand com `modeParams`/`teams`/`result`, mas a apresentação ficou
genérica: chips crus de chave:valor, o mesmo layout para qualquer arquétipo e o
jogador aparecendo até 3× (bloco PLAYERS da página + FORMAÇÃO + RESULTADO). O
operador quer olhar o expand e ler a partida de um jogo — e "partida" tem forma
diferente em cada arquétipo (ADR-0023).

Além disso o domínio já sabe QUEM ocupa cada papel num mundo host_world
(`RoleSlot.Occupants`), mas o DTO admin só expõe contagens — sem isso não há
como montar cards por papel com os jogadores atribuídos.

## Comportamento

### session-service / admin-bff (aditivo)

- `roleSlotDTO` (e o `RoleSlot` do BFF, passthrough 1:1) SHALL ganhar
  `occupants: []string` (`omitempty`) com os playerIDs presentes no papel.
  Payloads antigos sem o campo seguem válidos (front cai para contagem).

### admin-ui (redesign do `SessionMatchDetail`)

- O componente SHALL absorver o roster: as páginas (aba Sessions e board Rooms)
  não renderizam mais um bloco PLAYERS separado — cada jogador aparece **uma**
  vez, no contexto do painel. O bloco SLOTS POR PAPEL do expand do Rooms também
  é absorvido (a linha da tabela mantém o resumo `RoleSlots`).
- **Regras do modo**: o `modeParams` cru SHALL ser traduzido para o vocabulário
  da tela Configurações → Multiplayer (Estrutura, Critério, Formato, Empate,
  Lobby, Respawn, Bot-fill, Papéis externos…), com badge do arquétipo
  (`ARCHETYPE_META`) e o hash do snapshot. Arquétipo desconhecido cai em
  chave:valor genérico (não quebra).
- **Painel por arquétipo**:
  - `queue_arena` · teams → grid dinâmico (1 card por time, até 4 colunas):
    placar agregado do time, badge vencedor (troféu) / empate, linhas por
    jogador com pontos e stats individuais. Partida aberta = só a formação.
  - `queue_arena` · ffa → tabela ranqueada (vencedor primeiro, pontos desc).
  - `battle_royale` → tabela de colocação com pódio destacado (1º/2º/3º),
    coluna de squad colorida quando houver times, tempo de sobrevivência.
  - `coop_mission` → banner do desfecho coletivo (missão cumprida/falhou) +
    tabela única do time com stats individuais.
  - `host_world_dropin` → cards por papel: host (coroa) + um card por slot
    (co-op/invasor) com ocupantes nomeados (via `occupants`) e vagas livres
    tracejadas. Sem tabela de resultado (mundo não fecha por result).
  - Sessão legada (sem snapshot) → dispatch pelo `result.kind`; sem nada,
    roster simples (comportamento antigo preservado).
- Stats viram colunas próprias quando há ≤ 4 chaves distintas; acima disso
  (ou zero) caem numa coluna única agregada.
- Jogadores da sessão fora do resultado reportado aparecem em linha própria
  ("fora do resultado") — defensivo.

## Critérios de aceite

- [x] `occupants` no DTO admin (session) e no BFF, com testes; payload sem o
      campo segue decodando (compat)
- [x] `npm run build` (tsc + vite) verde; nenhuma página renderiza roster
      duplicado
- [x] e2e queue_arena teams: partida `teams-2v2` real → expand mostra regras
      legíveis + cards por time com placar agregado e vencedor
      (`expand-teams-2v2.png`)
- [x] Painéis BR/co-op/host_world validados com sessões sintéticas reais
      (modos temporários via API do registry + CreateSession/OpenWorld/
      ReportResult via grpcurl; screenshots `expand-{br-solo,coop-salvage,
      world-drop}.png`; modos de teste removidos ao fim)

## Fora do escopo

- Live view / streaming (descartado na 018)
- Novos dados no backend além de `occupants` (duração de rounds, timeline etc.)
- Resolução de handle dentro do resultado no BFF (segue join no front)
