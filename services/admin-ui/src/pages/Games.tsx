import { Alert, Button, Group, Loader, SimpleGrid, Stack, Text, Title, UnstyledButton } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useNavigate } from "react-router-dom";
import { IconPlus, IconRefresh } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { GameCover } from "../components/GameCover";
import { GameCreateWizard } from "./GameCreateWizard";

export function Games() {
  const nav = useNavigate();
  const { data: games, loading, error, reload } = useAsync((s) => api.games(s), []);
  const [opened, { open, close }] = useDisclosure(false);

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>Games</Title>
        <Group>
          <Button variant="default" leftSection={<IconRefresh size={16} />} onClick={reload}>
            Atualizar
          </Button>
          <Button leftSection={<IconPlus size={16} />} onClick={open}>
            Registrar jogo
          </Button>
        </Group>
      </Group>

      {loading && !games && <Loader />}
      {!!error && <Alert color="red">game-registry indisponível.</Alert>}
      {games && games.length === 0 && <Text c="dimmed">Nenhum jogo registrado ainda.</Text>}
      {games && games.length > 0 && (
        <SimpleGrid cols={{ base: 2, sm: 3, md: 4, lg: 6 }} spacing="md">
          {games.map((g) => (
            <UnstyledButton
              key={g.id}
              onClick={() => nav(`/games/${g.id}`)}
              style={{ transition: "transform 120ms ease" }}
              onMouseEnter={(e) => (e.currentTarget.style.transform = "scale(1.03)")}
              onMouseLeave={(e) => (e.currentTarget.style.transform = "scale(1)")}
            >
              <GameCover game={g} />
            </UnstyledButton>
          ))}
        </SimpleGrid>
      )}

      {/* Jornada de cadastro em steps (035) — substitui o modal slug+nome. */}
      <GameCreateWizard opened={opened} onClose={close} onCreated={reload} />
    </Stack>
  );
}
