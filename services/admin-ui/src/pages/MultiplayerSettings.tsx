import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Divider,
  Fieldset,
  Group,
  Loader,
  NumberInput,
  Select,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Switch,
  TagsInput,
  Text,
  Tooltip,
} from "@mantine/core";
import { IconInfoCircle } from "@tabler/icons-react";
import { notifications } from "@mantine/notifications";
import {
  api,
  HttpError,
  type MultiplayerProfile,
  type QueueArenaParams,
  type HostWorldDropInParams,
} from "../lib/api";

const STAT_RE = /^[a-z][a-z0-9_]*$/;

const QA_DEFAULTS: QueueArenaParams = {
  structure: "ffa",
  playerCount: 2,
  teamCount: 2,
  teamSize: 1,
  winCondition: "first_to_score",
  scoreTarget: 10,
  durationSeconds: 300,
  matchStructure: "single",
  roundsToWin: 2,
  allowDraw: false,
  scoringStats: [],
};

const HWD_DEFAULTS: HostWorldDropInParams = {
  enabledRoles: ["coop", "invader"],
  coopSlots: 2,
  invaderSlots: 1,
  levelRangeEnabled: true,
  weaponTierEnabled: false,
  passwordEnabled: true,
  invasionEnabled: true,
};

// FieldLabel renderiza um rótulo com ícone de ajuda e tooltip.
function FieldLabel({ label, help }: { label: string; help: string }) {
  return (
    <Tooltip label={help} position="right" multiline w={260} withArrow>
      <Group gap={4} style={{ display: "inline-flex", cursor: "default" }}>
        <Text size="sm" fw={500}>{label}</Text>
        <IconInfoCircle size={14} style={{ color: "var(--mantine-color-dimmed)" }} />
      </Group>
    </Tooltip>
  );
}

// SwitchWithHelp combina Switch com tooltip via label customizado.
function SwitchWithHelp({
  label,
  help,
  checked,
  onChange,
}: {
  label: string;
  help: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <Switch
      label={<FieldLabel label={label} help={help} />}
      checked={checked}
      onChange={(e) => onChange(e.currentTarget.checked)}
    />
  );
}

// buildQueueArenaProfile zera campos incoerentes com os discriminadores.
function buildQueueArenaProfile(topology: "dedicated" | "p2p", p: QueueArenaParams): MultiplayerProfile {
  const params: QueueArenaParams = {
    ...p,
    playerCount: p.structure === "ffa" ? p.playerCount : 0,
    teamCount: p.structure === "teams" ? p.teamCount : 0,
    teamSize: p.structure === "teams" ? p.teamSize : 0,
    scoreTarget: p.winCondition === "first_to_score" ? p.scoreTarget : 0,
    durationSeconds: p.winCondition === "highest_at_time" ? p.durationSeconds : 0,
    roundsToWin: p.matchStructure === "best_of_rounds" ? p.roundsToWin : 0,
  };
  return { archetype: "queue_arena", topology, params };
}

function buildHWDProfile(topology: "dedicated" | "p2p", p: HostWorldDropInParams): MultiplayerProfile {
  const hasCoop = p.enabledRoles.includes("coop");
  const hasInvader = p.enabledRoles.includes("invader");
  return {
    archetype: "host_world_dropin",
    topology,
    params: {
      ...p,
      coopSlots: hasCoop ? p.coopSlots : 0,
      invaderSlots: hasInvader ? p.invaderSlots : 0,
    },
  };
}

// --- Formulário do Queue Arena ---
function QueueArenaForm({
  p,
  set,
}: {
  p: QueueArenaParams;
  set: <K extends keyof QueueArenaParams>(k: K, v: QueueArenaParams[K]) => void;
}) {
  const invalidStats = p.scoringStats.filter((s) => !STAT_RE.test(s));

  return (
    <>
      <Fieldset legend="Composição">
        <Stack gap="sm">
          <div>
            <FieldLabel
              label="Estrutura"
              help="FFA: todos competem individualmente. Times: jogadores são divididos em equipes simétricas — o matchmaking atribui os lados."
            />
            <SegmentedControl
              mt={4}
              value={p.structure}
              onChange={(v) => set("structure", v as QueueArenaParams["structure"])}
              data={[
                { value: "ffa", label: "FFA (todos contra todos)" },
                { value: "teams", label: "Times" },
              ]}
            />
          </div>

          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
            {p.structure === "ffa" ? (
              <NumberInput
                label={
                  <FieldLabel
                    label="Jogadores na partida"
                    help="Total de jogadores formados por partida. O matchmaking aguarda exatamente este número antes de criar a sessão."
                  />
                }
                min={2}
                value={p.playerCount}
                onChange={(v) => set("playerCount", typeof v === "number" ? v : 2)}
              />
            ) : (
              <>
                <NumberInput
                  label={
                    <FieldLabel
                      label="Nº de times"
                      help="Quantos times disputam a partida. Mínimo 2 (ex.: 2 = NvN)."
                    />
                  }
                  min={2}
                  value={p.teamCount}
                  onChange={(v) => set("teamCount", typeof v === "number" ? v : 2)}
                />
                <NumberInput
                  label={
                    <FieldLabel
                      label="Jogadores por time"
                      help="Tamanho de cada time. Todos os times têm o mesmo tamanho (partida simétrica)."
                    />
                  }
                  min={1}
                  value={p.teamSize}
                  onChange={(v) => set("teamSize", typeof v === "number" ? v : 1)}
                />
              </>
            )}
          </SimpleGrid>
        </Stack>
      </Fieldset>

      <Fieldset legend="Condição de vitória">
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
          <Select
            label={
              <FieldLabel
                label="Critério"
                help="Define quando a partida termina e quem vence. O data plane reporta o resultado conforme este critério."
              />
            }
            value={p.winCondition}
            onChange={(v) => v && set("winCondition", v as QueueArenaParams["winCondition"])}
            allowDeselect={false}
            data={[
              { value: "first_to_score", label: "Primeiro a atingir a pontuação" },
              { value: "highest_at_time", label: "Maior pontuação ao fim do tempo" },
              { value: "objective", label: "Objetivo" },
              { value: "elimination", label: "Eliminação" },
            ]}
          />

          {p.winCondition === "first_to_score" ? (
            <NumberInput
              label={
                <FieldLabel
                  label="Pontuação-alvo"
                  help="O primeiro jogador/time a atingir este número de pontos vence a partida."
                />
              }
              min={1}
              value={p.scoreTarget}
              onChange={(v) => set("scoreTarget", typeof v === "number" ? v : 1)}
            />
          ) : p.winCondition === "highest_at_time" ? (
            <NumberInput
              label={
                <FieldLabel
                  label="Duração (segundos)"
                  help="Tempo total da partida. Ao esgotá-lo, quem tiver mais pontos vence."
                />
              }
              min={1}
              value={p.durationSeconds}
              onChange={(v) => set("durationSeconds", typeof v === "number" ? v : 60)}
            />
          ) : (
            <div />
          )}
        </SimpleGrid>
      </Fieldset>

      <Fieldset legend="Formato">
        <Stack gap="sm">
          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm" style={{ alignItems: "end" }}>
            <div>
              <FieldLabel
                label="Estrutura de rounds"
                help="Partida única: um confronto decide o vencedor. Melhor de N: série de rounds — vence quem ganhar mais rounds (ex.: melhor de 3 rounds)."
              />
              <SegmentedControl
                mt={4}
                value={p.matchStructure}
                onChange={(v) => set("matchStructure", v as QueueArenaParams["matchStructure"])}
                data={[
                  { value: "single", label: "Partida única" },
                  { value: "best_of_rounds", label: "Melhor de N rounds" },
                ]}
              />
            </div>

            {p.matchStructure === "best_of_rounds" ? (
              <NumberInput
                label={
                  <FieldLabel
                    label="Rounds para vencer"
                    help="Número de rounds necessários para ganhar a série. Ex.: 2 = melhor de 3 (primeiro a 2 rounds)."
                  />
                }
                min={1}
                value={p.roundsToWin}
                onChange={(v) => set("roundsToWin", typeof v === "number" ? v : 1)}
              />
            ) : (
              <div />
            )}
          </SimpleGrid>

          <SwitchWithHelp
            label="Permitir empate"
            help="Se habilitado, o resultado pode ser 'draw' além de 'win/loss'. Útil em esportes e modos com tempo fixo."
            checked={p.allowDraw}
            onChange={(v) => set("allowDraw", v)}
          />
        </Stack>
      </Fieldset>

      <Fieldset legend="Métricas">
        <TagsInput
          label={
            <FieldLabel
              label="Stats de pontuação"
              help="Métricas extras que o data plane reporta no resultado (ex.: kills, assists, flag_caps). Cada stat gera um leaderboard automático. Use identificadores em minúsculas começando com letra."
            />
          }
          description="Pressione Enter para adicionar cada stat."
          value={p.scoringStats}
          onChange={(v) => set("scoringStats", v)}
          error={
            invalidStats.length > 0
              ? `Stat inválido: ${invalidStats.join(", ")} (só minúsculas, começa com letra)`
              : undefined
          }
          clearable
        />
      </Fieldset>
    </>
  );
}

// --- Formulário do Host World Drop-in ---
function HostWorldDropInForm({
  p,
  set,
}: {
  p: HostWorldDropInParams;
  set: <K extends keyof HostWorldDropInParams>(k: K, v: HostWorldDropInParams[K]) => void;
}) {
  const hasCoop = p.enabledRoles.includes("coop");
  const hasInvader = p.enabledRoles.includes("invader");

  function toggleRole(role: "coop" | "invader", checked: boolean) {
    const next = checked
      ? [...p.enabledRoles, role]
      : p.enabledRoles.filter((r) => r !== role);
    set("enabledRoles", next as HostWorldDropInParams["enabledRoles"]);
  }

  return (
    <>
      <Fieldset legend="Papéis e capacidade">
        <Stack gap="sm">
          <div>
            <FieldLabel
              label="Papéis externos habilitados"
              help="Define quem pode entrar no mundo do host. Co-op: aliados que ajudam. Invasor: adversários que entram sem convite (se invasão habilitada)."
            />
            <Group mt="xs" gap="md">
              <Checkbox
                label="Co-op (aliado)"
                checked={hasCoop}
                onChange={(e) => toggleRole("coop", e.currentTarget.checked)}
              />
              <Checkbox
                label="Invasor (adversário)"
                checked={hasInvader}
                onChange={(e) => toggleRole("invader", e.currentTarget.checked)}
              />
            </Group>
            {p.enabledRoles.length === 0 && (
              <Text size="xs" c="red" mt={4}>
                Pelo menos um papel deve estar habilitado.
              </Text>
            )}
          </div>

          <SimpleGrid cols={{ base: 1, xs: 2 }} spacing="sm">
            {hasCoop && (
              <NumberInput
                label={
                  <FieldLabel
                    label="Slots de co-op"
                    help="Máximo de aliados simultâneos no mundo. Tipicamente 2–3."
                  />
                }
                min={1}
                max={8}
                value={p.coopSlots}
                onChange={(v) => set("coopSlots", typeof v === "number" ? v : 1)}
              />
            )}
            {hasInvader && (
              <NumberInput
                label={
                  <FieldLabel
                    label="Slots de invasor"
                    help="Máximo de invasores simultâneos. Tipicamente 1–2."
                  />
                }
                min={1}
                max={4}
                value={p.invaderSlots}
                onChange={(v) => set("invaderSlots", typeof v === "number" ? v : 1)}
              />
            )}
          </SimpleGrid>
        </Stack>
      </Fieldset>

      <Fieldset legend="Critérios de matchmaking">
        <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="sm">
          <SwitchWithHelp
            label="Faixa de nível"
            help="Filtra quem pode entrar pelo nível do personagem. Co-ops são limitados a uma faixa próxima do host; invasores têm faixa assimétrica (ligeiramente acima). Desativar permite qualquer nível."
            checked={p.levelRangeEnabled}
            onChange={(v) => set("levelRangeEnabled", v)}
          />
          <SwitchWithHelp
            label="Tier de equipamento"
            help="Adiciona filtro pelo nível de arma/equipamento além do nível do personagem. Evita que jogadores com gear muito superior ou inferior entrem."
            checked={p.weaponTierEnabled}
            onChange={(v) => set("weaponTierEnabled", v)}
          />
          <SwitchWithHelp
            label="Senha de sessão"
            help="Permite que host e phantoms usem uma senha compartilhada que ignora as faixas de nível e gear. Útil para jogar com amigos independente de progressão."
            checked={p.passwordEnabled}
            onChange={(v) => set("passwordEnabled", v)}
          />
        </SimpleGrid>
      </Fieldset>

      <Fieldset legend="Comportamento">
        <SwitchWithHelp
          label="Invasão automática"
          help="Se habilitado, o mundo do host fica elegível para invasores sem convite — invasores procuram mundos disponíveis e entram ativamente. Se desabilitado, o mundo aceita apenas co-op (aliados invocados pelo host)."
          checked={p.invasionEnabled}
          onChange={(v) => set("invasionEnabled", v)}
        />
      </Fieldset>
    </>
  );
}

// --- Componente principal ---
export function MultiplayerSettings({ gameId }: { gameId: string }) {
  const [loading, setLoading] = useState(true);
  const [configured, setConfigured] = useState(false);
  const [topology, setTopology] = useState<"dedicated" | "p2p">("dedicated");
  const [archetype, setArchetype] = useState<"queue_arena" | "host_world_dropin">("queue_arena");
  const [qa, setQA] = useState<QueueArenaParams>(QA_DEFAULTS);
  const [hwd, setHWD] = useState<HostWorldDropInParams>(HWD_DEFAULTS);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const ctrl = new AbortController();
    setLoading(true);
    api
      .getMultiplayerProfile(gameId, ctrl.signal)
      .then((profile) => {
        if (profile) {
          setConfigured(true);
          setTopology(profile.topology);
          setArchetype(profile.archetype);
          if (profile.archetype === "queue_arena") {
            setQA({ ...QA_DEFAULTS, ...profile.params });
          } else {
            setHWD({ ...HWD_DEFAULTS, ...profile.params });
          }
        } else {
          setConfigured(false);
          setTopology("dedicated");
          setArchetype("queue_arena");
          setQA(QA_DEFAULTS);
          setHWD(HWD_DEFAULTS);
        }
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return;
        notifications.show({ color: "red", message: "Falha ao carregar o perfil de multiplayer." });
        console.error(err);
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false);
      });
    return () => ctrl.abort();
  }, [gameId]);

  function setQAField<K extends keyof QueueArenaParams>(key: K, value: QueueArenaParams[K]) {
    setQA((prev) => ({ ...prev, [key]: value }));
  }

  function setHWDField<K extends keyof HostWorldDropInParams>(key: K, value: HostWorldDropInParams[K]) {
    setHWD((prev) => ({ ...prev, [key]: value }));
  }

  const invalidQAStats = qa.scoringStats.filter((s) => !STAT_RE.test(s));
  const hwdRolesEmpty = archetype === "host_world_dropin" && hwd.enabledRoles.length === 0;
  const canSave = invalidQAStats.length === 0 && !hwdRolesEmpty && !saving;

  async function save() {
    setSaving(true);
    try {
      const profile =
        archetype === "queue_arena"
          ? buildQueueArenaProfile(topology, qa)
          : buildHWDProfile(topology, hwd);
      await api.setMultiplayerProfile(gameId, profile);
      setConfigured(true);
      notifications.show({ color: "green", message: "Perfil de multiplayer salvo." });
    } catch (err) {
      const code = err instanceof HttpError ? err.code : undefined;
      notifications.show({
        color: "red",
        message: code ? `Falha ao salvar (${code}).` : "Falha ao salvar (parâmetros inválidos ou serviço fora?).",
      });
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <Loader size="sm" />;

  return (
    <Stack maw={860}>
      {!configured && (
        <Alert color="blue" variant="light">
          Este jogo ainda não tem um perfil de multiplayer. Defina abaixo e salve para configurá-lo.
        </Alert>
      )}

      <Fieldset legend="Arquétipo e topologia">
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
          <Select
            label={
              <FieldLabel
                label="Arquétipo"
                help="Define o modelo de multiplayer do jogo. Queue Arena: partida simétrica formada por fila (FFA ou NvN). Host World Drop-in: mundo persistente de um host onde phantoms entram com papéis assimétricos (estilo Soulslike)."
              />
            }
            value={archetype}
            onChange={(v) => v && setArchetype(v as typeof archetype)}
            allowDeselect={false}
            data={[
              { value: "queue_arena", label: "Queue Arena — partida simétrica (FFA / NvN)" },
              { value: "host_world_dropin", label: "Host World Drop-in — mundo persistente (Soulslike)" },
            ]}
          />

          <Tooltip label="p2p é reservado (ADR-0023) e ainda não implementado — o servidor rejeita.">
            <Select
              label={
                <FieldLabel
                  label="Topologia"
                  help="Dedicated Server: o gameplay roda num servidor dedicado (Agones/Unity). Autoridade neutra, sem trapaças do lado do cliente. P2P: o host lógico hospeda o mundo localmente — sem custo de servidor mas com risco de cheating."
                />
              }
              value={topology}
              onChange={(v) => v && setTopology(v as "dedicated" | "p2p")}
              allowDeselect={false}
              data={[
                { value: "dedicated", label: "Dedicated Server" },
                { value: "p2p", label: "P2P (reservado — não implementado)" },
              ]}
            />
          </Tooltip>
        </SimpleGrid>
      </Fieldset>

      <Divider label="Configurações do arquétipo" labelPosition="left" />

      {archetype === "queue_arena" ? (
        <QueueArenaForm p={qa} set={setQAField} />
      ) : (
        <HostWorldDropInForm p={hwd} set={setHWDField} />
      )}

      <Group>
        <Button onClick={save} disabled={!canSave} loading={saving}>
          {configured ? "Salvar" : "Configurar multiplayer"}
        </Button>
      </Group>
    </Stack>
  );
}
