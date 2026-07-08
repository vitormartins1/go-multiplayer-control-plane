# Tasks — 019-admin-session-archetype-layout

- [x] session-service: `roleSlotDTO.occupants` (omitempty) + teste transport
- [x] admin-bff: `domain.RoleSlot.Occupants` + decode no client + teste
- [x] admin-ui: `RoleSlot.occupants?` em `lib/api.ts`
- [x] admin-ui: `SessionMatchDetail` reescrito — regras legíveis por arquétipo
      (vocabulário da tela de Configurações) + painel por arquétipo (team cards /
      tabela ranqueada / colocação com pódio / banner coletivo / cards por papel)
- [x] admin-ui: páginas GameSessions e Rooms deixam de renderizar PLAYERS/SLOTS
      duplicados (roster vive no painel)
- [x] Verificação: `go test -race` 303 verdes (session + bff) · `npm run build`
      verde · e2e `teams-2v2` real (k3d) · sessões sintéticas br-solo/
      coop-salvage/world-drop via grpcurl · screenshots headless (puppeteer-core
      + Brave) das 4 variantes no expand
