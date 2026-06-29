import { Alert, Button, Group, Loader, Modal, Stack, Table, Text, TextInput, Title } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { useNavigate } from "react-router-dom";
import { IconPlus, IconRefresh } from "@tabler/icons-react";
import { api, HttpError } from "../lib/api";
import { useAsync } from "../lib/useAsync";

const SLUG_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

export function Games() {
  const nav = useNavigate();
  const { data: games, loading, error, reload } = useAsync((s) => api.games(s), []);
  const [opened, { open, close }] = useDisclosure(false);

  const form = useForm({
    initialValues: { slug: "", name: "" },
    validate: {
      slug: (v) => (SLUG_RE.test(v.trim()) ? null : "slug minúsculo, sem espaços (a-z 0-9 -)"),
      name: (v) => (v.trim() ? null : "obrigatório"),
    },
  });

  async function submit(values: { slug: string; name: string }) {
    try {
      await api.createGame(values.slug.trim(), values.name.trim());
      notifications.show({ color: "green", message: "Jogo registrado." });
      close();
      form.reset();
      reload();
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

      {loading && !games && <Loader />}
      {!!error && <Alert color="red">game-registry indisponível.</Alert>}
      {games && games.length === 0 && <Text c="dimmed">Nenhum jogo registrado ainda.</Text>}
      {games && games.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Nome</Table.Th>
              <Table.Th>Slug</Table.Th>
              <Table.Th>Criado</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {games.map((g) => (
              <Table.Tr key={g.id} style={{ cursor: "pointer" }} onClick={() => nav(`/games/${g.id}`)}>
                <Table.Td>{g.name}</Table.Td>
                <Table.Td c="dimmed">{g.slug}</Table.Td>
                <Table.Td>{new Date(g.createdAt).toLocaleDateString("pt-BR")}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
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
