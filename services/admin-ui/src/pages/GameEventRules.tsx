import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Group,
  Loader,
  Modal,
  Select,
  Stack,
  Table,
  TagsInput,
  Text,
  TextInput,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { IconPlus, IconRefresh, IconTrash } from "@tabler/icons-react";
import { useState } from "react";
import { api, HttpError, type EventRule, type MultiplayerProfile } from "../lib/api";
import { useAsync } from "../lib/useAsync";

const ARCHETYPES: MultiplayerProfile["archetype"][] = [
  "host_world_dropin",
  "queue_arena",
  "battle_royale",
  "coop_mission",
];

// MatchPair é uma linha do editor de "facts" (match) — livre por natureza: os
// facts variam por arquétipo (role/event/outcome no host_world_dropin;
// placement/tier no battle_royale; outcome nos demais), então não dá pra
// hardcodar campos fixos como no form de achievements.
type MatchPair = { key: string; value: string };

function matchLabel(match: Record<string, string>): string {
  const entries = Object.entries(match);
  return entries.length === 0 ? "(sempre)" : entries.map(([k, v]) => `${k}=${v}`).join(", ");
}

// GameEventRules é o CRUD das regras de mapeamento evento->achievement-event-
// type (migration 008 do achievements-service) — substitui as antigas funções
// Go hardcoded por arquétipo (ParticipationEventTypes/PlacementEventTypes/
// CollectiveEventTypes). Regras globais (uma por arquétipo, semeadas na
// migration) são read-only aqui; só overrides do jogo são criáveis/removíveis.
export function GameEventRules({ gameId }: { gameId: string }) {
  const { data: rules, loading, error, reload } = useAsync((s) => api.gameEventRules(gameId, s), [gameId]);
  const [opened, { open, close }] = useDisclosure(false);
  const [deleting, setDeleting] = useState<EventRule | null>(null);
  const [matchPairs, setMatchPairs] = useState<MatchPair[]>([{ key: "", value: "" }]);

  const form = useForm({
    initialValues: {
      archetype: "host_world_dropin" as MultiplayerProfile["archetype"],
      eventTypes: [] as string[],
    },
    validate: {
      eventTypes: (v) => (v.length === 0 ? "obrigatório, ao menos um" : null),
    },
  });

  function openCreate() {
    form.reset();
    setMatchPairs([{ key: "", value: "" }]);
    open();
  }

  function addMatchRow() {
    setMatchPairs((p) => [...p, { key: "", value: "" }]);
  }
  function removeMatchRow(i: number) {
    setMatchPairs((p) => p.filter((_, idx) => idx !== i));
  }
  function updateMatchRow(i: number, field: "key" | "value", v: string) {
    setMatchPairs((p) => p.map((row, idx) => (idx === i ? { ...row, [field]: v } : row)));
  }

  async function submit(values: typeof form.values) {
    const match: Record<string, string> = {};
    for (const { key, value } of matchPairs) {
      if (key.trim()) match[key.trim()] = value.trim();
    }
    try {
      await api.createEventRule(gameId, { archetype: values.archetype, match, eventTypes: values.eventTypes });
      notifications.show({ color: "green", message: "Regra adicionada." });
      close();
      reload();
    } catch (e) {
      const msg =
        e instanceof HttpError && e.status === 400
          ? "Dados inválidos (confira archetype/eventTypes)."
          : "Falha ao adicionar (serviço indisponível?).";
      form.setErrors({ eventTypes: msg });
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await api.deleteEventRule(gameId, deleting.id);
      notifications.show({ color: "green", message: "Regra removida." });
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
        <Text fw={600}>Regras de mapeamento evento → conquista</Text>
        <Group>
          <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reload}>
            Atualizar
          </Button>
          <Button size="xs" leftSection={<IconPlus size={14} />} onClick={openCreate}>
            Nova regra
          </Button>
        </Group>
      </Group>

      <Text size="sm" c="dimmed">
        Cada regra casa um subconjunto exato de "facts" do evento recebido (ex.: role/event/outcome no
        host_world_dropin; placement/tier no battle_royale; outcome nos demais) e soma os event types listados
        ao progresso das conquistas cadastradas na aba Achievements. Regras <b>globais</b> valem pra todo jogo
        com o arquétipo (semeadas por padrão, não editáveis aqui); regras do <b>jogo</b> só valem pra este jogo.
      </Text>

      {loading && !rules && <Loader />}
      {!!error && <Alert color="red">achievements indisponível.</Alert>}
      {rules && rules.length === 0 && <Text c="dimmed">Nenhuma regra cadastrada.</Text>}
      {rules && rules.length > 0 && (
        <Table striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Escopo</Table.Th>
              <Table.Th>Arquétipo</Table.Th>
              <Table.Th>Match</Table.Th>
              <Table.Th>Event types</Table.Th>
              <Table.Th w={50}></Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {rules.map((r) => (
              <Table.Tr key={r.id}>
                <Table.Td>
                  <Badge variant="outline" color={r.scope === "global" ? "gray" : "teal"}>
                    {r.scope === "global" ? "global" : "jogo"}
                  </Badge>
                </Table.Td>
                <Table.Td c="dimmed">{r.archetype}</Table.Td>
                <Table.Td>
                  {Object.keys(r.match).length === 0 ? (
                    <Text c="dimmed" size="sm">
                      (sempre)
                    </Text>
                  ) : (
                    <Group gap={4} wrap="wrap">
                      {Object.entries(r.match).map(([k, v]) => (
                        <Badge key={k} variant="light" color="gray">
                          {k}={v}
                        </Badge>
                      ))}
                    </Group>
                  )}
                </Table.Td>
                <Table.Td>
                  <Group gap={4} wrap="wrap">
                    {r.eventTypes.map((et) => (
                      <Badge key={et} variant="light" color="violet">
                        {et}
                      </Badge>
                    ))}
                  </Group>
                </Table.Td>
                <Table.Td>
                  {r.scope === "game" && (
                    <ActionIcon variant="subtle" color="red" onClick={() => setDeleting(r)} aria-label="Remover">
                      <IconTrash size={16} />
                    </ActionIcon>
                  )}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      {/* Nova regra */}
      <Modal opened={opened} onClose={close} title="Nova regra" size="lg">
        <form onSubmit={form.onSubmit(submit)}>
          <Stack>
            <Select label="Arquétipo" data={ARCHETYPES} allowDeselect={false} {...form.getInputProps("archetype")} />
            <Stack gap={4}>
              <Text size="sm" fw={500}>
                Match (facts do evento; vazio = sempre bate)
              </Text>
              {matchPairs.map((row, i) => (
                <Group key={i} gap={6} wrap="nowrap">
                  <TextInput
                    placeholder="chave (ex.: role)"
                    value={row.key}
                    onChange={(e) => updateMatchRow(i, "key", e.currentTarget.value)}
                    style={{ flex: 1 }}
                  />
                  <TextInput
                    placeholder="valor (ex.: invader)"
                    value={row.value}
                    onChange={(e) => updateMatchRow(i, "value", e.currentTarget.value)}
                    style={{ flex: 1 }}
                  />
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    onClick={() => removeMatchRow(i)}
                    aria-label="Remover par"
                    disabled={matchPairs.length === 1}
                  >
                    <IconTrash size={16} />
                  </ActionIcon>
                </Group>
              ))}
              <Button variant="default" size="xs" onClick={addMatchRow}>
                + par chave/valor
              </Button>
            </Stack>
            <TagsInput
              label="Event types"
              description="Pressione Enter para adicionar cada um."
              placeholder="custom_event_type"
              {...form.getInputProps("eventTypes")}
            />
            <Group justify="flex-end">
              <Button type="submit">Adicionar</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Confirmar remoção */}
      <Modal opened={!!deleting} onClose={() => setDeleting(null)} title="Remover regra">
        <Stack>
          <Text size="sm">
            Remover a regra <code>{deleting?.archetype}</code> ({deleting ? matchLabel(deleting.match) : ""})?
            Esta ação não pode ser desfeita.
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
