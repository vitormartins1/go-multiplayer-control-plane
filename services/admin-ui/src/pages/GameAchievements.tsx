import {
  ActionIcon,
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
import { IconPencil, IconPlus, IconRefresh, IconTrash } from "@tabler/icons-react";
import { useState } from "react";
import { api, HttpError, type CatalogAchievement } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { AchievementIcon } from "../components/AchievementIcon";

const ID_RE = /^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$/;

export function GameAchievements({ gameId }: { gameId: string }) {
  const { data: catalog, loading, error, reload } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);
  const [opened, { open, close }] = useDisclosure(false);
  const [editing, setEditing] = useState<CatalogAchievement | null>(null);
  const [deleting, setDeleting] = useState<CatalogAchievement | null>(null);

  const form = useForm({
    initialValues: { id: "", name: "", description: "", eventType: "", target: 1 },
    validate: {
      id: (v) => (ID_RE.test(v.trim()) ? null : "id minúsculo (ex.: first_win)"),
      name: (v) => (v.trim() ? null : "obrigatório"),
      eventType: (v) => (v.trim() ? null : "obrigatório"),
    },
  });

  // form de edição: sem id (a chave não muda).
  const editForm = useForm({
    initialValues: { name: "", description: "", eventType: "", target: 1 },
    validate: {
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

  function openEdit(a: CatalogAchievement) {
    editForm.setValues({ name: a.name, description: a.description, eventType: a.eventType, target: a.target });
    setEditing(a);
  }

  async function submitEdit(values: typeof editForm.values) {
    if (!editing) return;
    try {
      await api.updateAchievement(gameId, editing.id, { ...values, target: Number(values.target) || 1 });
      notifications.show({ color: "green", message: "Conquista atualizada." });
      setEditing(null);
      reload();
    } catch (e) {
      const msg =
        e instanceof HttpError && e.status === 404
          ? "Conquista não existe mais."
          : e instanceof HttpError && e.status === 400
            ? "Dados inválidos."
            : "Falha ao salvar (serviço indisponível?).";
      editForm.setErrors({ name: msg });
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await api.deleteAchievement(gameId, deleting.id);
      notifications.show({ color: "green", message: "Conquista removida." });
      setDeleting(null);
      reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao remover (serviço indisponível?)." });
      setDeleting(null);
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
              <Table.Th w={40}></Table.Th>
              <Table.Th>Id</Table.Th>
              <Table.Th>Nome</Table.Th>
              <Table.Th>Descrição</Table.Th>
              <Table.Th>Evento</Table.Th>
              <Table.Th>Meta</Table.Th>
              <Table.Th w={90}></Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {catalog.map((a) => (
              <Table.Tr key={a.id}>
                <Table.Td>
                  <AchievementIcon seed={a.id} />
                </Table.Td>
                <Table.Td c="dimmed">{a.id}</Table.Td>
                <Table.Td>{a.name}</Table.Td>
                <Table.Td c="dimmed">{a.description}</Table.Td>
                <Table.Td c="dimmed">{a.eventType}</Table.Td>
                <Table.Td>{a.target}</Table.Td>
                <Table.Td>
                  <Group gap={4} wrap="nowrap">
                    <ActionIcon variant="subtle" color="gray" onClick={() => openEdit(a)} aria-label="Editar">
                      <IconPencil size={16} />
                    </ActionIcon>
                    <ActionIcon variant="subtle" color="red" onClick={() => setDeleting(a)} aria-label="Remover">
                      <IconTrash size={16} />
                    </ActionIcon>
                  </Group>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      {/* Nova conquista */}
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

      {/* Editar conquista (id é a chave, não editável) */}
      <Modal opened={!!editing} onClose={() => setEditing(null)} title={`Editar ${editing?.id ?? ""}`}>
        <form onSubmit={editForm.onSubmit(submitEdit)}>
          <Stack>
            <TextInput label="Nome" {...editForm.getInputProps("name")} />
            <Textarea label="Descrição" autosize minRows={2} {...editForm.getInputProps("description")} />
            <TextInput label="Event type" {...editForm.getInputProps("eventType")} />
            <NumberInput label="Meta (target)" min={1} {...editForm.getInputProps("target")} />
            <Group justify="flex-end">
              <Button type="submit">Salvar</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Confirmar remoção */}
      <Modal opened={!!deleting} onClose={() => setDeleting(null)} title="Remover conquista">
        <Stack>
          <Text size="sm">
            Remover <b>{deleting?.name}</b> (<code>{deleting?.id}</code>)? O progresso dos jogadores nesta
            conquista também será apagado. Esta ação não pode ser desfeita.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleting(null)}>
              Cancelar
            </Button>
            <Button color="red" onClick={confirmDelete}>
              Remover
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
