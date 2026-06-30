import { Alert, Button, Group, Loader, Modal, SimpleGrid, Stack, Text, TextInput, Title, UnstyledButton } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { useNavigate } from "react-router-dom";
import { IconPlus, IconRefresh } from "@tabler/icons-react";
import { api, HttpError } from "../lib/api";
import { useGames } from "../lib/games";
import { GameCover } from "../components/GameCover";

const SLUG_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

export function Games() {
  const nav = useNavigate();
  const { games, loading, error, reload, setActiveGameId } = useGames();
  const [opened, { open, close }] = useDisclosure(false);

  // openGame seleciona o jogo como ativo (contexto global) e abre o detalhe.
  function openGame(id: string) {
    setActiveGameId(id);
    nav(`/games/${id}`);
  }

  const form = useForm({
    initialValues: { slug: "", name: "" },
    validate: {
      slug: (v) => (SLUG_RE.test(v.trim()) ? null : "slug minúsculo, sem espaços (a-z 0-9 -)"),
      name: (v) => (v.trim() ? null : "obrigatório"),
    },
  });

  async function submit(values: { slug: string; name: string }) {
    try {
      const game = await api.createGame(values.slug.trim(), values.name.trim());
      notifications.show({ color: "green", message: "Jogo registrado." });
      close();
      form.reset();
      reload();
      setActiveGameId(game.id); // novo jogo vira o ativo
    } catch (e) {
      const msg =
        e instanceof HttpError && e.status === 409
          ? "Slug já existe."
          : e instanceof HttpError && e.status === 400
            ? "Dados inválidos."
            : "Falha ao registrar (serviço indisponível?).";
      form.setErrors({ slug: msg });
    }
  }

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

      {loading && games.length === 0 && <Loader />}
      {!!error && <Alert color="red">game-registry indisponível.</Alert>}
      {!loading && games.length === 0 && <Text c="dimmed">Nenhum jogo registrado ainda.</Text>}
      {games.length > 0 && (
        <SimpleGrid cols={{ base: 2, sm: 3, md: 4, lg: 6 }} spacing="md">
          {games.map((g) => (
            <UnstyledButton
              key={g.id}
              onClick={() => openGame(g.id)}
              style={{ transition: "transform 120ms ease" }}
              onMouseEnter={(e) => (e.currentTarget.style.transform = "scale(1.03)")}
              onMouseLeave={(e) => (e.currentTarget.style.transform = "scale(1)")}
            >
              <GameCover game={g} />
            </UnstyledButton>
          ))}
        </SimpleGrid>
      )}

      <Modal opened={opened} onClose={close} title="Registrar jogo">
        <form onSubmit={form.onSubmit(submit)}>
          <Stack>
            <TextInput label="Slug" placeholder="meu-jogo" {...form.getInputProps("slug")} />
            <TextInput label="Nome" placeholder="Meu Jogo" {...form.getInputProps("name")} />
            <Group justify="flex-end">
              <Button type="submit">Registrar</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </Stack>
  );
}
