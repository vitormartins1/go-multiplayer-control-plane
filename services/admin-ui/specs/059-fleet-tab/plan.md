# Plan 059 — aba Fleet

## Backend: `admin-bff` (proxy do campo `fleet`, hoje ausente)

Mesmo padrão do `RenameGame` (rename de jogo, spec 010) — mesma forma, endpoint próprio (o game-registry já usa `PUT /games/{id}/fleet` separado do `PUT /games/{id}` genérico, spec 012).

### `internal/domain/admin.go`

```go
type Game struct {
	ID                  string    `json:"id"`
	Slug                string    `json:"slug"`
	Name                string    `json:"name"`
	CreatedAt           time.Time `json:"createdAt"`
	Image               string    `json:"image"`
	ImageSource         string    `json:"imageSource"`
	GeneratedImageSeed  string    `json:"generatedImageSeed"`
	GeneratedImageStyle string    `json:"generatedImageStyle"`
	// Fleet Agones do jogo (ADR-0026); "" = sem Fleet cadastrada. Só agora
	// exposto pelo BFF — o game-registry já devolve no GET /games desde a
	// spec 012, mas o campo nunca foi propagado até aqui (spec 059).
	Fleet string `json:"fleet"`
}
```

`GameRegistry` (mesma interface, perto de `RenameGame`):

```go
	// SetFleet cadastra/troca a Fleet Agones do jogo (ADR-0026). UpstreamError
	// (400/404) propagado. "" limpa o campo (sem Agones real habilitado).
	SetFleet(ctx context.Context, id, fleet string) (*Game, error)
```

### `internal/gameregistry/client.go`

```go
// SetFleet cadastra/troca a Fleet Agones do jogo (PUT /games/{id}/fleet,
// spec 012 do game-registry — endpoint próprio, separado do PUT genérico).
func (c *Client) SetFleet(ctx context.Context, id, fleet string) (*domain.Game, error) {
	body, _ := json.Marshal(map[string]string{"fleet": fleet})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/games/"+url.PathEscape(id)+"/fleet", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := upstream.Do(c.http, req, domain.ErrGameRegistryUnavailable)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var g domain.Game
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("gameregistry: decode: %w", err)
	}
	return &g, nil
}
```

`ListGames`/`CreateGame`/`RenameGame` não mudam — já decodificam pra `domain.Game`, então `fleet` passa a vir preenchido de graça assim que o campo existir na struct.

### `internal/service/admin_service.go`

```go
// SetFleet cadastra/troca a Fleet Agones do jogo (Configurações do painel, spec 059).
func (s *Service) SetFleet(ctx context.Context, id, fleet string) (*domain.Game, error) {
	return s.registry.SetFleet(ctx, id, fleet)
}
```

### `internal/transport/http_handler.go`

- `AdminService` (interface local do pacote): adicionar `SetFleet(ctx context.Context, id, fleet string) (*domain.Game, error)` perto de `RenameGame`.
- Rota (perto de `PUT /api/games/{gameId}`, linha ~79): `mux.HandleFunc("PUT /api/games/{gameId}/fleet", h.setFleet)`.
- Handler (mesmo corpo do `renameGame`, campo trocado):

```go
// setFleet cadastra/troca a Fleet Agones do jogo (spec 059).
func (h *Handler) setFleet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Fleet string `json:"fleet"`
	}
	if err := decode(r, &req); err != nil {
		httpkit.WriteError(w, http.StatusBadRequest, "bad_request")
		return
	}
	g, err := h.svc.SetFleet(r.Context(), r.PathValue("gameId"), req.Fleet)
	if err != nil {
		writeUpstream(w, err)
		return
	}
	httpkit.WriteJSON(w, http.StatusOK, g)
}
```

### Testes (admin-bff)

- `gameregistry/client_test.go`: `TestSetFleet_RepassaAoRegistry` (httptest, mesmo padrão de `TestRenameGame_...`).
- `service/admin_service_test.go`: `TestSetFleet_Delega` (mock de `GameRegistry`).
- `transport/http_handler_test.go`: `TestSetFleet_Sucesso200`, `TestSetFleet_CorpoInvalido_400`, `TestSetFleet_UpstreamIndisponivel_502` (ou o status já convencionado por `writeUpstream`/`ErrGameRegistryUnavailable` nos outros testes de rename, espelhar).

## Frontend: `admin-ui`

### `src/lib/api.ts`

```ts
export interface Game {
  id: string;
  slug: string;
  name: string;
  createdAt: string;
  image?: string;
  imageSource?: string;
  generatedImageSeed?: string;
  generatedImageStyle?: string;
  multiplayerModes?: ModeSummary[];
  // Fleet Agones (ADR-0026); "" = sem fleet cadastrada.
  fleet?: string;
}
```

```ts
  setFleet: (gameId: string, fleet: string) =>
    putJSON<Game>(`/api/games/${encodeURIComponent(gameId)}/fleet`, { fleet }),
```

### `src/pages/Fleet.tsx` (novo)

Estrutura: alerta fixo de disclaimer no topo + tabela de jogos (mesmo kit de `Table striped highlightOnHover withTableBorder` + expander já usado em `GameSessions.tsx`/`GameAchievements.tsx`).

```tsx
import { Alert, Badge, Box, Button, Collapse, Group, Select, Stack, Table, Text, TextInput, Tooltip } from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import { useState } from "react";
import { api, type Game } from "../lib/api";
import { useAsync } from "../lib/useAsync";

// mockFleetStatus (Fase 0, spec 059): dado ILUSTRATIVO determinístico por
// nome de fleet — sem chamada de rede. A integração real (GHCR + leitura de
// estado do Agones) não existe ainda; ver "Visão de evolução futura" na spec.
function mockFleetStatus(fleetName: string) {
  const seed = [...fleetName].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7);
  const minor = 3 + (seed % 3);
  const patch = seed % 5;
  const currentTag = `v0.${minor}.${patch}`;
  const total = 2 + (seed % 3);
  return {
    currentTag,
    healthy: total,
    total,
    availableTags: [currentTag, `v0.${minor}.${patch + 1}`, `v0.${minor + 1}.0-rc1`],
  };
}

export function Fleet() {
  const { data: games, reload } = useAsync((s) => api.games(s), []);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  return (
    <Stack>
      <Text size="lg" fw={600}>Fleet</Text>
      <Alert color="yellow" icon={<IconAlertTriangle size={16} />}>
        Imagem, versão e health desta tela são <b>ilustrativos</b> — a integração real
        (build publicado num registry + leitura de estado do Agones) ainda não existe.
        Ver spec 059.
      </Alert>

      {games && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Jogo</Table.Th>
              <Table.Th>Fleet</Table.Th>
              <Table.Th w={90}></Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {games.map((g) => (
              <FleetRow
                key={g.id}
                game={g}
                expanded={expandedId === g.id}
                onToggle={() => setExpandedId(expandedId === g.id ? null : g.id)}
                onSaved={reload}
              />
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}

function FleetRow({
  game,
  expanded,
  onToggle,
  onSaved,
}: {
  game: Game;
  expanded: boolean;
  onToggle: () => void;
  onSaved: () => void;
}) {
  const [fleetInput, setFleetInput] = useState(game.fleet ?? "");
  const [saving, setSaving] = useState(false);
  const mock = game.fleet ? mockFleetStatus(game.fleet) : null;

  async function save() {
    setSaving(true);
    try {
      await api.setFleet(game.id, fleetInput.trim());
      onSaved();
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <Table.Tr style={{ cursor: "pointer" }} onClick={onToggle}>
        <Table.Td>{game.name}</Table.Td>
        <Table.Td>
          {game.fleet ? <Badge variant="light">{game.fleet}</Badge> : <Text c="dimmed" size="sm">sem fleet configurada</Text>}
        </Table.Td>
        <Table.Td />
      </Table.Tr>
      {expanded && (
        <Table.Tr>
          <Table.Td colSpan={3} p={0}>
            <Collapse in={expanded}>
              <Box p="md">
                <Stack gap="sm">
                  <Group align="flex-end">
                    <TextInput
                      label="Fleet Agones"
                      placeholder="packet-arena"
                      value={fleetInput}
                      onChange={(e) => setFleetInput(e.currentTarget.value)}
                      w={260}
                    />
                    <Button onClick={save} loading={saving}>Salvar</Button>
                  </Group>

                  {mock && (
                    <Group gap="xl" wrap="wrap" mt="xs">
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>IMAGEM ATUAL</Text>
                        <Text size="sm" ff="monospace">{mock.currentTag} (mock)</Text>
                      </Stack>
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>HEALTH</Text>
                        <Badge color="green" variant="light">{mock.healthy}/{mock.total} saudáveis (mock)</Badge>
                      </Stack>
                      <Stack gap={2}>
                        <Text size="xs" c="dimmed" fw={600}>VERSÕES DISPONÍVEIS (mock)</Text>
                        <Group gap={6}>
                          <Select data={mock.availableTags} defaultValue={mock.currentTag} w={160} size="xs" />
                          <Tooltip label="Integração com o registry de imagens ainda não implementada">
                            <Button size="xs" variant="light" disabled>Promover</Button>
                          </Tooltip>
                        </Group>
                      </Stack>
                    </Group>
                  )}
                </Stack>
              </Box>
            </Collapse>
          </Table.Td>
        </Table.Tr>
      )}
    </>
  );
}
```

Notas:
- `mockFleetStatus` só roda quando `game.fleet` existe — jogos sem fleet não mostram a seção mockada (item da spec: "não faz sentido mockar build de uma fleet que não existe").
- `Tooltip` + `disabled` no botão "Promover" — nunca dá `onClick`, nunca finge sucesso (critério de aceite da spec).
- Sem estado que sobrevive a reload: `expandedId`/`fleetInput` são só `useState` local, perdidos ao navegar/recarregar.
- Reusa `useAsync`/`Table`/`Collapse` já estabelecidos — nenhum componente/lib novo.

### `src/App.tsx`

```tsx
const Fleet = lazy(() => import("./pages/Fleet").then((m) => ({ default: m.Fleet })));

const NAV = [
  { to: "/games", label: "Games", icon: IconDeviceGamepad2 },
  { to: "/players", label: "Players", icon: IconUsers },
  { to: "/rooms", label: "Rooms", icon: IconDoorEnter },
  { to: "/analytics", label: "Analytics", icon: IconChartBar },
  { to: "/fleet", label: "Fleet", icon: IconServer2 }, // confirmado disponível na versão instalada de @tabler/icons-react; distinto do IconServer já usado em Infrastructure
  { to: "/infrastructure", label: "Infrastructure", icon: IconServer },
];
```

+ `<Route path="/fleet" element={<Fleet />} />` no bloco de `<Routes>`.

Não mexe em `GameSettings.tsx` — a edição do fleet passa a viver só na aba Fleet (evita duplicar a mesma responsabilidade em dois lugares).

## Testes / verificação

Backend: `go test ./services/admin-bff/... -race` (unit, os 3 níveis acima) + `go build ./...`.

Frontend: sem suíte automatizada (padrão já estabelecido nas specs anteriores). `tsc --noEmit` + `npm run build`, smoke manual via Playwright:
1. Nav mostra "Fleet"; navegar lá lista os jogos.
2. Jogo com fleet cadastrada hoje (Packet Arena) aparece com o nome real + expande mostrando a seção mockada (com o disclaimer visível).
3. Jogo sem fleet aparece com "sem fleet configurada" e sem seção mockada.
4. Editar o campo Fleet de um jogo (real) → reload da página confirma persistência (via `GET /games` de novo).
5. Clicar em "Promover" não dispara nenhuma request de rede (Network tab do Playwright/DevTools limpo) — botão sempre desabilitado.
6. `tsc --noEmit` + `npm run build` limpos.
