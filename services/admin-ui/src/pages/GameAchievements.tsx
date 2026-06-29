import {
  Alert,
  Button,
  Group,
  Loader,
  Modal,
  NumberInput,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { IconPlus, IconRefresh } from "@tabler/icons-react";
import { api, HttpError } from "../lib/api";
import { useAsync } from "../lib/useAsync";

const ID_RE = /^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$/;

export function GameAchievements({ gameId }: { gameId: string }) {
  const { data: catalog, loading, error, reload } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);
  const [opened, { open, close }] = useDisclosure(false);

  const form = useForm({
    initialValues: { id: "", name: "", description: "", eventType: "", target: 1 },
    validate: {
      id: (v) => (ID_RE.test(v.trim()) ? null : "id minúsculo (ex.: first_win)"),
      name: (v) => (v.trim() ? null : "obrigatório"),
      eventType: (v) => (v.trim() ? null : "obrigatório"),
    },
  });

  async function submit(values: typeof form.values) {
    try {
      await api.createAchievement(gameId, { ...values, target: Number(values.target) || 1 });
      notifications.show({ color: "green", message: "Conquista adicionada." });
      close();
      form.reset();
      reload();
    } catch (e) {
      const msg =
        e instanceof HttpError && e.status === 409
          ? "Já existe uma conquista com esse id neste jogo."
          : e instanceof HttpError && e.status === 400
            ? "Dados inválidos."
            : "Falha ao adicionar (serviço indisponível?).";
      form.setErrors({ id: msg });
    }
  }

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>Catálogo de conquistas</Text>
        <Group>
          <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
            Atualizar
          </Button>
          <Button size="xs" leftSection={<IconPlus size={14} />} onClick={open}>
            Nova conquista
          </Button>
        </Group>
      </Group>

      {loading && !catalog && <Loader />}
      {!!error && <Alert color="red">achievements indisponível.</Alert>}
      {catalog && catalog.length === 0 && <Text c="dimmed">Nenhuma conquista neste jogo ainda.</Text>}
      {catalog && catalog.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Id</Table.Th>
              <Table.Th>Nome</Table.Th>
              <Table.Th>Descrição</Table.Th>
              <Table.Th>Meta</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {catalog.map((a) => (
              <Table.Tr key={a.id}>
                <Table.Td c="dimmed">{a.id}</Table.Td>
                <Table.Td>{a.name}</Table.Td>
                <Table.Td c="dimmed">{a.description}</Table.Td>
                <Table.Td>{a.target}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      <Modal opened={opened} onClose={close} title="Nova conquista">
        <form onSubmit={form.onSubmit(submit)}>
          <Stack>
            <TextInput label="Id (slug)" placeholder="first_win" {...form.getInputProps("id")} />
            <TextInput label="Nome" placeholder="Primeira Vitória" {...form.getInputProps("name")} />
            <Textarea label="Descrição" autosize minRows={2} {...form.getInputProps("description")} />
            <TextInput label="Event type" placeholder="match_won" {...form.getInputProps("eventType")} />
            <NumberInput label="Meta (target)" min={1} {...form.getInputProps("target")} />
            <Group justify="flex-end">
              <Button type="submit">Adicionar</Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </Stack>
  );
}
