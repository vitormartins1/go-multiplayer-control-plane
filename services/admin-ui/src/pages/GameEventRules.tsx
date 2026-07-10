import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  CloseButton,
  Group,
  Loader,
  Modal,
  ScrollArea,
  Stack,
  Table,
  Text,
  TextInput,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconBan, IconRefresh, IconRotate2, IconSearch, IconTrash } from "@tabler/icons-react";
import { useState } from "react";
import { api, type EventRule, type MultiplayerProfile } from "../lib/api";
import { condLabel } from "../lib/matchCond";
import { gameArchetypes, filterRulesForGame } from "../lib/eventRules";
import { useAsync } from "../lib/useAsync";
import { GameEventRulesCanvas } from "./GameEventRulesCanvas";

// Fallback só pro caso raro de o jogo ainda não ter NENHUM modo/perfil
// multiplayer cadastrado (dropdown não pode ficar vazio) — usado só pro
// seletor de arquétipo interno do canvas (archetypeOptions).
const ALL_ARCHETYPES: MultiplayerProfile["archetype"][] = [
  "host_world_dropin",
  "queue_arena",
  "battle_royale",
  "coop_mission",
];

function matchLabel(match: EventRule["match"]): string {
  const entries = Object.entries(match);
  return entries.length === 0 ? "(sempre)" : entries.map(([k, v]) => condLabel(k, v)).join(", ");
}

// matchesListFilter (spec 055): filtro de texto da lista — busca em
// arquétipo, chaves/valores do match e event types. Case-insensitive.
function matchesListFilter(term: string) {
  const t = term.trim().toLowerCase();
  return (r: EventRule) => {
    if (!t) return true;
    if (r.archetype.toLowerCase().includes(t)) return true;
    if (r.eventTypes.some((et) => et.toLowerCase().includes(t))) return true;
    return Object.entries(r.match).some(
      ([k, v]) => k.toLowerCase().includes(t) || String(v).toLowerCase().includes(t),
    );
  };
}

// GameEventRules é o CRUD das regras de mapeamento evento->achievement-event-
// type (migration 008 do achievements-service) — substitui as antigas funções
// Go hardcoded por arquétipo (ParticipationEventTypes/PlacementEventTypes/
// CollectiveEventTypes). Regras globais (uma por arquétipo, semeadas na
// migration) são read-only aqui; só overrides do jogo são criáveis/removíveis.
export function GameEventRules({ gameId }: { gameId: string }) {
  const { data: rules, loading, error, reload } = useAsync((s) => api.gameEventRules(gameId, s), [gameId]);
  // modos/perfis do jogo — a fonte de verdade de quais arquétipos ele
  // realmente usa (evita listar/oferecer arquétipos que o jogo não tem).
  const { data: modes } = useAsync((s) => api.listMultiplayerModes(gameId, s), [gameId]);
  // catálogo de achievements — 4ª camada do canvas (quem escuta cada type).
  const { data: catalog, reload: reloadCatalog } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);
  // Atualizar recarrega as DUAS fontes do canvas — um achievement recém-criado
  // na aba Achievements precisa aparecer na camada 4 sem trocar de aba.
  function reloadAll() {
    reload();
    reloadCatalog();
  }
  const archetypes = gameArchetypes(modes ?? []);
  // Sem nenhum modo cadastrado ainda: cai pra lista fixa (dropdown não pode
  // ficar vazio) — ver filterRulesForGame pro mesmo raciocínio na listagem.
  const archetypeOptions = archetypes.size > 0 ? [...archetypes] : ALL_ARCHETYPES;
  const visibleRules = filterRulesForGame(rules ?? [], archetypes);

  // Lista + canvas lado a lado (spec 055): a lista reaproveita as colunas da
  // antiga view "table"; selecionar uma linha foca o canvas no subgrafo dela
  // (canvas fica vazio até uma seleção — resolve a poluição visual com muitos
  // achievements, achado no estresse-teste de 66 regras).
  const [focusRuleId, setFocusRuleId] = useState<string | null>(null);
  const [listFilter, setListFilter] = useState("");
  const filteredRules = visibleRules.filter(matchesListFilter(listFilter));

  const [deleting, setDeleting] = useState<EventRule | null>(null);

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

  // toggleSuppress: paridade com o canvas (spec achievements/017, V2b) —
  // mesma ação, mesma reversibilidade sem modal.
  async function toggleSuppress(r: EventRule) {
    try {
      if (r.suppressed) {
        await api.unsuppressEventRule(gameId, r.id);
        notifications.show({ color: "green", message: "Regra restaurada." });
      } else {
        await api.suppressEventRule(gameId, r.id);
        notifications.show({ color: "orange", message: "Regra suprimida para este jogo." });
      }
      reload();
    } catch {
      notifications.show({ color: "red", message: "Falha ao atualizar a supressão (serviço indisponível?)." });
    }
  }

  return (
    <Stack>
      <Group justify="space-between">
        <Text fw={600}>Regras de mapeamento evento → conquista</Text>
        <Button variant="default" size="xs" leftSection={<IconRefresh size={14} />} onClick={reloadAll}>
          Atualizar
        </Button>
      </Group>

      <Text size="sm" c="dimmed">
        Cada regra casa um subconjunto exato de "facts" do evento recebido (ex.: role/event/outcome no
        host_world_dropin; placement/tier no battle_royale; outcome nos demais) e soma os event types listados ao
        progresso das conquistas cadastradas na aba Achievements. Regras <b>globais</b> valem pra todo jogo com o
        arquétipo (semeadas por padrão, não editáveis aqui) — só as dos arquétipos que <b>este jogo</b> tem
        cadastrado aparecem abaixo; regras do <b>jogo</b> só valem pra este jogo. Selecione uma regra na lista pra
        ver o subgrafo dela no canvas.
      </Text>

      {loading && !rules && <Loader />}
      {!!error && <Alert color="red">achievements indisponível.</Alert>}
      {rules && visibleRules.length === 0 && (
        <Text c="dimmed">Nenhuma regra cadastrada para os arquétipos deste jogo.</Text>
      )}

      {rules && visibleRules.length > 0 && (
        <Group align="flex-start" gap="md" wrap="nowrap">
          <Box w={380} style={{ flexShrink: 0 }}>
            <TextInput
              size="xs"
              mb={6}
              placeholder="filtrar regras…"
              leftSection={<IconSearch size={13} />}
              value={listFilter}
              onChange={(e) => setListFilter(e.currentTarget.value)}
              rightSection={listFilter ? <CloseButton size="xs" onClick={() => setListFilter("")} /> : undefined}
            />
            <ScrollArea h="calc(100vh - 420px)" type="auto">
              <Table striped highlightOnHover withTableBorder stickyHeader>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Escopo</Table.Th>
                    <Table.Th>Event types</Table.Th>
                    <Table.Th w={40}></Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {filteredRules.map((r) => (
                    <Table.Tr
                      key={r.id}
                      onClick={() => setFocusRuleId((cur) => (cur === r.id ? null : r.id))}
                      style={{
                        cursor: "pointer",
                        opacity: r.suppressed ? 0.5 : undefined,
                        background: r.id === focusRuleId ? "var(--mantine-color-blue-light)" : undefined,
                      }}
                    >
                      <Table.Td>
                        <Stack gap={4}>
                          <Group gap={4} wrap="nowrap">
                            <Badge variant="outline" color={r.scope === "global" ? "gray" : "teal"}>
                              {r.scope === "global" ? "global" : "jogo"}
                            </Badge>
                            {r.suppressed && (
                              <Badge variant="light" color="orange" leftSection={<IconBan size={10} />}>
                                suprimida
                              </Badge>
                            )}
                          </Group>
                          <Text size="xs" c="dimmed">
                            {r.archetype}
                          </Text>
                        </Stack>
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
                      <Table.Td onClick={(e) => e.stopPropagation()}>
                        {r.scope === "game" ? (
                          <ActionIcon variant="subtle" color="red" onClick={() => setDeleting(r)} aria-label="Remover">
                            <IconTrash size={16} />
                          </ActionIcon>
                        ) : (
                          <ActionIcon
                            variant="subtle"
                            color={r.suppressed ? "teal" : "orange"}
                            onClick={() => toggleSuppress(r)}
                            aria-label={r.suppressed ? "Restaurar" : "Suprimir"}
                          >
                            {r.suppressed ? <IconRotate2 size={16} /> : <IconBan size={16} />}
                          </ActionIcon>
                        )}
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </ScrollArea>
          </Box>
          <Box style={{ flex: 1, minWidth: 0 }}>
            <GameEventRulesCanvas
              gameId={gameId}
              rules={visibleRules}
              catalog={catalog ?? []}
              archetypeOptions={archetypeOptions}
              modes={modes ?? []}
              reload={reloadAll}
              focusRuleId={focusRuleId}
              onFocusRule={setFocusRuleId}
            />
          </Box>
        </Group>
      )}

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
