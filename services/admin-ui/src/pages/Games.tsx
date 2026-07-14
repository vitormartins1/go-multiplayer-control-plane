import { Alert, Button, Group, Loader, SimpleGrid, Stack, Text, TextInput, Title, UnstyledButton } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { IconPlus, IconRefresh, IconSearch } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { GameCover } from "../components/GameCover";
import { GameCreateWizard } from "./GameCreateWizard";
import "./Games.css";

// Games (design review spec 082 — não passava por revisão desde o registro
// inicial do projeto, só ganhou os badges de arquétipo e o wizard de
// registro depois): título com contagem (mesmo padrão de Players.tsx),
// busca por nome/slug (idem), hover do card via CSS em vez de JS.
export function Games() {
  const nav = useNavigate();
  const { data: games, loading, error, reload } = useAsync((s) => api.games(s), []);
  const [opened, { open, close }] = useDisclosure(false);
  const [search, setSearch] = useState("");

  const filtered =
    games?.filter((g) => {
      const term = search.trim().toLowerCase();
      return term === "" || g.name.toLowerCase().includes(term) || g.slug.toLowerCase().includes(term);
    }) ?? [];

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>
          Games{" "}
          {games && (
            <Text span c="dimmed">
              · {games.length}
            </Text>
          )}
        </Title>
        <Group>
          <Button variant="default" leftSection={<IconRefresh size={16} />} onClick={reload}>
            Atualizar
          </Button>
          <Button leftSection={<IconPlus size={16} />} onClick={open}>
            Registrar jogo
          </Button>
        </Group>
      </Group>

      {games && games.length > 1 && (
        <TextInput
          placeholder="Filtrar por nome ou slug…"
          leftSection={<IconSearch size={16} />}
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
          w={280}
        />
      )}

      {loading && !games && <Loader />}
      {!!error && <Alert color="critical">game-registry indisponível.</Alert>}
      {games && games.length === 0 && <Text c="dimmed">Nenhum jogo registrado ainda.</Text>}
      {games && games.length > 0 && filtered.length === 0 && (
        <Text c="dimmed">Nenhum resultado para este filtro.</Text>
      )}
      {filtered.length > 0 && (
        <SimpleGrid cols={{ base: 2, sm: 3, md: 4, lg: 6 }} spacing="md">
          {filtered.map((g) => (
            <UnstyledButton key={g.id} className="game-card" onClick={() => nav(`/games/${g.id}`)}>
              <GameCover game={g} modes={g.multiplayerModes} />
            </UnstyledButton>
          ))}
        </SimpleGrid>
      )}

      {/* Jornada de cadastro em steps (035) — substitui o modal slug+nome. */}
      <GameCreateWizard opened={opened} onClose={close} onCreated={reload} />
    </Stack>
  );
}
