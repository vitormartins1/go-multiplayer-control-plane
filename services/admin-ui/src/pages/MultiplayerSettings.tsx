import { useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Checkbox,
  Code,
  Divider,
  Fieldset,
  Group,
  Loader,
  Modal,
  NumberInput,
  Select,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Switch,
  Table,
  TagsInput,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import { IconInfoCircle, IconPencil, IconPlus, IconTrash } from "@tabler/icons-react";
import { notifications } from "@mantine/notifications";
import {
  api,
  DEFAULT_MODE_ID,
  HttpError,
  MODE_ID_RE,
  type MultiplayerMode,
  type MultiplayerProfile,
  type QueueArenaParams,
  type HostWorldDropInParams,
  type BattleRoyaleParams,
  type CoopMissionParams,
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

const BR_DEFAULTS: BattleRoyaleParams = {
  lobbySize: 60,
  teamSize: 1,
  respawnMode: "one_life",
  scoringStats: [],
};

const CM_DEFAULTS: CoopMissionParams = {
  teamSize: 4,
  missions: [],
  difficulties: [],
  botfillEnabled: true,
  scoringStats: [],
};

const RESPAWN_OPTIONS: { value: BattleRoyaleParams["respawnMode"]; label: string }[] = [
  { value: "one_life", label: "Uma vida (BR clássico)" },
  { value: "teammate_revive", label: "Revive por aliado" },
  { value: "arena_1v1", label: "Arena 1v1 (Gulag)" },
  { value: "reboot_timer", label: "Reboot com janela" },
];

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

function buildBattleRoyaleProfile(topology: "dedicated" | "p2p", p: BattleRoyaleParams): MultiplayerProfile {
  return { archetype: "battle_royale", topology, params: { ...p } };
}

function buildCoopMissionProfile(topology: "dedicated" | "p2p", p: CoopMissionParams): MultiplayerProfile {
  return { archetype: "coop_mission", topology, params: { ...p } };
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
// Layout: dois SimpleGrid externos de 2 colunas (md+).
//   Row 1: [Composição | Condição de vitória]
//   Row 2: [Formato    | Métricas           ]
// Campos dentro de cada Fieldset sempre em Stack (sem grids internos).
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
      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Fieldset legend="Composição">
          <Stack gap="sm">
            <div>
              <FieldLabel
                label="Estrutura"
                help="FFA: todos competem individualmente. Times: jogadores são divididos em equipes simétricas — o matchmaking atribui os lados."
              />
              <SegmentedControl
                mt={4}
                fullWidth
                value={p.structure}
                onChange={(v) => set("structure", v as QueueArenaParams["structure"])}
                data={[
                  { value: "ffa", label: "FFA (todos vs todos)" },
                  { value: "teams", label: "Times" },
                ]}
              />
            </div>

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
          </Stack>
        </Fieldset>

        <Fieldset legend="Condição de vitória">
          <Stack gap="sm">
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

            {p.winCondition === "first_to_score" && (
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
            )}

            {p.winCondition === "highest_at_time" && (
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
            )}
          </Stack>
        </Fieldset>
      </SimpleGrid>

      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Fieldset legend="Formato">
          <Stack gap="sm">
            <div>
              <FieldLabel
                label="Estrutura de rounds"
                help="Partida única: um confronto decide o vencedor. Melhor de N: série de rounds — vence quem ganhar mais rounds (ex.: melhor de 3 rounds)."
              />
              <SegmentedControl
                mt={4}
                fullWidth
                value={p.matchStructure}
                onChange={(v) => set("matchStructure", v as QueueArenaParams["matchStructure"])}
                data={[
                  { value: "single", label: "Partida única" },
                  { value: "best_of_rounds", label: "Melhor de N" },
                ]}
              />
            </div>

            {p.matchStructure === "best_of_rounds" && (
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
            )}

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
      </SimpleGrid>
    </>
  );
}

// --- Formulário do Host World Drop-in ---
// Layout:
//   [Papéis e capacidade — full width]
//   SimpleGrid 2-col (md+): [Critérios de matchmaking | Comportamento]
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

          {(hasCoop || hasInvader) && (
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
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
          )}
        </Stack>
      </Fieldset>

      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Fieldset legend="Critérios de matchmaking">
          <Stack gap="sm">
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
          </Stack>
        </Fieldset>

        <Fieldset legend="Comportamento">
          <Stack gap="sm">
            <SwitchWithHelp
              label="Invasão automática"
              help="Se habilitado, o mundo do host fica elegível para invasores sem convite — invasores procuram mundos disponíveis e entram ativamente. Se desabilitado, o mundo aceita apenas co-op (aliados invocados pelo host)."
              checked={p.invasionEnabled}
              onChange={(v) => set("invasionEnabled", v)}
            />
          </Stack>
        </Fieldset>
      </SimpleGrid>
    </>
  );
}

// --- Formulário do Battle Royale ---
// Layout: SimpleGrid 2-col (md+): [Lobby e squads | Respawn] + [Métricas — full].
function BattleRoyaleForm({
  p,
  set,
}: {
  p: BattleRoyaleParams;
  set: <K extends keyof BattleRoyaleParams>(k: K, v: BattleRoyaleParams[K]) => void;
}) {
  const invalidStats = p.scoringStats.filter((s) => !STAT_RE.test(s));
  const minLobby = 2 * p.teamSize;
  const lobbyTooSmall = p.lobbySize < minLobby;

  return (
    <>
      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Fieldset legend="Lobby e squads">
          <Stack gap="sm">
            <NumberInput
              label={
                <FieldLabel
                  label="Tamanho do lobby"
                  help="Alvo de jogadores por partida (ex.: 60). O matchmaking enche o lobby até este número antes de abrir a sessão. Deve ser ao menos 2× o tamanho do squad."
                />
              }
              min={2}
              value={p.lobbySize}
              onChange={(v) => set("lobbySize", typeof v === "number" ? v : 2)}
              error={lobbyTooSmall ? `Deve ser ≥ ${minLobby} (2 × squad)` : undefined}
            />
            <NumberInput
              label={
                <FieldLabel
                  label="Tamanho do squad"
                  help="Jogadores por time. 1 = solo (FFA); 2 = duo; 4 = squad. O matchmaking agrupa em squads; o último pode ficar menor."
                />
              }
              min={1}
              value={p.teamSize}
              onChange={(v) => set("teamSize", typeof v === "number" ? v : 1)}
            />
          </Stack>
        </Fieldset>

        <Fieldset legend="Respawn">
          <Select
            label={
              <FieldLabel
                label="Modo de respawn"
                help="Decide quando a colocação é fixada (comportamento do data plane; o backend só armazena). Uma vida: sem volta. Revive por aliado: derrubado ≠ eliminado. Arena 1v1: quem morre disputa um 1v1 pra voltar. Reboot: volta se o time pegar um item a tempo."
              />
            }
            value={p.respawnMode}
            onChange={(v) => v && set("respawnMode", v as BattleRoyaleParams["respawnMode"])}
            allowDeselect={false}
            data={RESPAWN_OPTIONS}
          />
        </Fieldset>
      </SimpleGrid>

      <Fieldset legend="Métricas">
        <TagsInput
          label={
            <FieldLabel
              label="Stats de pontuação"
              help="Métricas extras que o data plane reporta no resultado além da colocação e da sobrevivência (ex.: kills). Cada stat gera um leaderboard automático. Use identificadores em minúsculas começando com letra."
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

// --- Formulário do Co-op Mission ---
// Layout: SimpleGrid 2-col (md+): [Time e bots | Catálogo (missões/dificuldades)] + [Métricas — full].
function CoopMissionForm({
  p,
  set,
}: {
  p: CoopMissionParams;
  set: <K extends keyof CoopMissionParams>(k: K, v: CoopMissionParams[K]) => void;
}) {
  const invalidStats = p.scoringStats.filter((s) => !STAT_RE.test(s));
  const invalidMissions = p.missions.filter((s) => !STAT_RE.test(s));
  const invalidDiffs = p.difficulties.filter((s) => !STAT_RE.test(s));

  return (
    <>
      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Fieldset legend="Time e bots">
          <Stack gap="sm">
            <NumberInput
              label={
                <FieldLabel
                  label="Tamanho do time"
                  help="Jogadores no time co-op (ex.: 4). Todos enfrentam o ambiente juntos e recebem o mesmo desfecho — sucesso ou falha da missão."
                />
              }
              min={2}
              value={p.teamSize}
              onChange={(v) => set("teamSize", typeof v === "number" ? v : 2)}
            />
            <SwitchWithHelp
              label="Bot-fill"
              help="Se vagas humanas são preenchidas por bots (comportamento do data plane; o backend só armazena a flag). Permite jogar a missão sem o time cheio."
              checked={p.botfillEnabled}
              onChange={(v) => set("botfillEnabled", v)}
            />
          </Stack>
        </Fieldset>

        <Fieldset legend="Catálogo (data plane)">
          <Stack gap="sm">
            <TagsInput
              label={
                <FieldLabel
                  label="Missões"
                  help="Catálogo de missões que o host pode escolher (identificadores estáveis, ex.: salvage_op). O conteúdo real da missão mora no data plane; aqui só declaramos os identificadores. Pode ficar vazio."
                />
              }
              description="Enter para adicionar. Identificadores em minúsculas."
              value={p.missions}
              onChange={(v) => set("missions", v)}
              error={
                invalidMissions.length > 0
                  ? `Inválido: ${invalidMissions.join(", ")} (só minúsculas, começa com letra)`
                  : undefined
              }
              clearable
            />
            <TagsInput
              label={
                <FieldLabel
                  label="Dificuldades"
                  help="Catálogo de dificuldades (ex.: hazard_1, hazard_5). Filtra a experiência, não o desfecho. Identificadores estáveis; pode ficar vazio."
                />
              }
              description="Enter para adicionar. Identificadores em minúsculas."
              value={p.difficulties}
              onChange={(v) => set("difficulties", v)}
              error={
                invalidDiffs.length > 0
                  ? `Inválido: ${invalidDiffs.join(", ")} (só minúsculas, começa com letra)`
                  : undefined
              }
              clearable
            />
          </Stack>
        </Fieldset>
      </SimpleGrid>

      <Fieldset legend="Métricas">
        <TagsInput
          label={
            <FieldLabel
              label="Stats de pontuação"
              help="Métricas por jogador que o resultado carrega (ex.: kills, revives, ore_mined). Modulam recompensa/XP — não mudam o desfecho coletivo. Cada stat gera um leaderboard automático. Use identificadores em minúsculas."
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

// --- Rótulos por arquétipo (lista de modos) ---
const ARCHETYPE_META: Record<
  MultiplayerProfile["archetype"],
  { label: string; color: string }
> = {
  queue_arena: { label: "Queue Arena", color: "blue" },
  host_world_dropin: { label: "Host World Drop-in", color: "grape" },
  battle_royale: { label: "Battle Royale", color: "orange" },
  coop_mission: { label: "Co-op Mission", color: "teal" },
};

// --- Editor de um modo (ADR-0024, spec 023) ---
// Reusa o form por arquétipo como corpo; a moldura adiciona a identidade do
// modo (modeId estável + displayName). modeId trava na edição (rename de
// exibição = displayName) e no primeiro cadastro (CTA cria o "default").
function ModeEditor({
  gameId,
  mode,
  lockDefault,
  existingModeIds,
  onSaved,
  onCancel,
}: {
  gameId: string;
  mode: MultiplayerMode | null; // null = criar
  lockDefault: boolean; // primeiro cadastro: modeId = "default" travado
  existingModeIds: string[];
  onSaved: () => void;
  onCancel: () => void;
}) {
  const isEdit = mode !== null;
  const [modeId, setModeId] = useState(mode?.modeId ?? (lockDefault ? DEFAULT_MODE_ID : ""));
  const [displayName, setDisplayName] = useState(mode?.displayName ?? "");
  const [topology, setTopology] = useState<"dedicated" | "p2p">(mode?.topology ?? "dedicated");
  const [archetype, setArchetype] = useState<MultiplayerProfile["archetype"]>(
    mode?.archetype ?? "queue_arena",
  );
  const [qa, setQA] = useState<QueueArenaParams>(
    mode?.archetype === "queue_arena" ? { ...QA_DEFAULTS, ...mode.params } : QA_DEFAULTS,
  );
  const [hwd, setHWD] = useState<HostWorldDropInParams>(
    mode?.archetype === "host_world_dropin" ? { ...HWD_DEFAULTS, ...mode.params } : HWD_DEFAULTS,
  );
  const [br, setBR] = useState<BattleRoyaleParams>(
    mode?.archetype === "battle_royale" ? { ...BR_DEFAULTS, ...mode.params } : BR_DEFAULTS,
  );
  const [cm, setCM] = useState<CoopMissionParams>(
    mode?.archetype === "coop_mission" ? { ...CM_DEFAULTS, ...mode.params } : CM_DEFAULTS,
  );
  const [saving, setSaving] = useState(false);

  function setQAField<K extends keyof QueueArenaParams>(key: K, value: QueueArenaParams[K]) {
    setQA((prev) => ({ ...prev, [key]: value }));
  }
  function setHWDField<K extends keyof HostWorldDropInParams>(key: K, value: HostWorldDropInParams[K]) {
    setHWD((prev) => ({ ...prev, [key]: value }));
  }
  function setBRField<K extends keyof BattleRoyaleParams>(key: K, value: BattleRoyaleParams[K]) {
    setBR((prev) => ({ ...prev, [key]: value }));
  }
  function setCMField<K extends keyof CoopMissionParams>(key: K, value: CoopMissionParams[K]) {
    setCM((prev) => ({ ...prev, [key]: value }));
  }

  const modeIdLocked = isEdit || lockDefault;
  const modeIdInvalid = !MODE_ID_RE.test(modeId);
  const modeIdTaken = !modeIdLocked && existingModeIds.includes(modeId);
  const displayNameTooLong = displayName.trim().length > 60;
  const invalidQAStats = qa.scoringStats.filter((st) => !STAT_RE.test(st));
  const hwdRolesEmpty = archetype === "host_world_dropin" && hwd.enabledRoles.length === 0;
  const invalidBRStats = br.scoringStats.filter((st) => !STAT_RE.test(st));
  const brLobbyInvalid = archetype === "battle_royale" && br.lobbySize < 2 * br.teamSize;
  const cmFieldsInvalid =
    archetype === "coop_mission" &&
    (cm.teamSize < 2 ||
      [...cm.scoringStats, ...cm.missions, ...cm.difficulties].some((st) => !STAT_RE.test(st)));
  const canSave =
    !modeIdInvalid &&
    !modeIdTaken &&
    !displayNameTooLong &&
    invalidQAStats.length === 0 &&
    !hwdRolesEmpty &&
    invalidBRStats.length === 0 &&
    !brLobbyInvalid &&
    !cmFieldsInvalid &&
    !saving;

  async function save() {
    setSaving(true);
    try {
      const profile =
        archetype === "queue_arena"
          ? buildQueueArenaProfile(topology, qa)
          : archetype === "host_world_dropin"
            ? buildHWDProfile(topology, hwd)
            : archetype === "battle_royale"
              ? buildBattleRoyaleProfile(topology, br)
              : buildCoopMissionProfile(topology, cm);
      const name = displayName.trim();
      await api.setMultiplayerMode(gameId, modeId, {
        ...profile,
        ...(name ? { displayName: name } : {}),
      });
      notifications.show({ color: "green", message: `Modo "${name || modeId}" salvo.` });
      onSaved();
    } catch (err) {
      const code = err instanceof HttpError ? err.code : undefined;
      notifications.show({
        color: "red",
        message: code
          ? `Falha ao salvar (${code}).`
          : "Falha ao salvar (parâmetros inválidos ou serviço fora?).",
      });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Stack>
      <Fieldset legend="Identidade do modo">
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
          <TextInput
            label={
              <FieldLabel
                label="ID do modo"
                help="Identificador estável do modo (vai para matchmaking/token/eventos no M1). Minúsculas, dígitos e hífen, começando com letra; máx. 32. Imutável após criado — para renomear a exibição use o nome abaixo."
              />
            }
            placeholder="ex.: tdm, ranked, warzone"
            value={modeId}
            onChange={(e) => setModeId(e.currentTarget.value)}
            disabled={modeIdLocked}
            error={
              !modeIdLocked && modeId.length > 0 && modeIdInvalid
                ? "Formato: ^[a-z][a-z0-9-]{0,31}$"
                : modeIdTaken
                  ? "Já existe um modo com este id."
                  : undefined
            }
          />
          <TextInput
            label={
              <FieldLabel
                label="Nome de exibição"
                help="Nome amigável do modo mostrado no console (opcional, até 60 caracteres). Pode mudar a qualquer momento."
              />
            }
            placeholder="ex.: Team Deathmatch"
            value={displayName}
            onChange={(e) => setDisplayName(e.currentTarget.value)}
            error={displayNameTooLong ? "Máximo de 60 caracteres." : undefined}
          />
        </SimpleGrid>
      </Fieldset>

      <Fieldset legend="Arquétipo e topologia">
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
          <Select
            label={
              <FieldLabel
                label="Arquétipo"
                help="Define o modelo de multiplayer do modo. Queue Arena: partida simétrica formada por fila (FFA ou NvN). Host World Drop-in: mundo persistente de um host onde phantoms entram com papéis assimétricos (estilo Soulslike). Battle Royale: lobby grande, colocação ordinal. Co-op Mission: time pequeno vs. ambiente, resultado coletivo (L4D/Deep Rock)."
              />
            }
            value={archetype}
            onChange={(v) => v && setArchetype(v as typeof archetype)}
            allowDeselect={false}
            data={[
              { value: "queue_arena", label: "Queue Arena — partida simétrica (FFA / NvN)" },
              { value: "host_world_dropin", label: "Host World Drop-in — mundo persistente (Soulslike)" },
              { value: "battle_royale", label: "Battle Royale — lobby grande, colocação (PUBG/Fortnite)" },
              { value: "coop_mission", label: "Co-op Mission — time vs. ambiente, resultado coletivo (L4D/Deep Rock)" },
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
      ) : archetype === "host_world_dropin" ? (
        <HostWorldDropInForm p={hwd} set={setHWDField} />
      ) : archetype === "battle_royale" ? (
        <BattleRoyaleForm p={br} set={setBRField} />
      ) : (
        <CoopMissionForm p={cm} set={setCMField} />
      )}

      <Group>
        <Button onClick={save} disabled={!canSave} loading={saving}>
          Salvar modo
        </Button>
        <Button variant="default" onClick={onCancel} disabled={saving}>
          Cancelar
        </Button>
      </Group>
    </Stack>
  );
}

// --- Componente principal: coleção de modos do jogo (ADR-0024) ---
type View =
  | { kind: "list" }
  | { kind: "create"; lockDefault: boolean }
  | { kind: "edit"; mode: MultiplayerMode };

export function MultiplayerSettings({ gameId }: { gameId: string }) {
  const [loading, setLoading] = useState(true);
  const [modes, setModes] = useState<MultiplayerMode[]>([]);
  const [view, setView] = useState<View>({ kind: "list" });
  const [deleting, setDeleting] = useState<MultiplayerMode | null>(null);
  const [removing, setRemoving] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const ctrl = new AbortController();
    setLoading(true);
    api
      .listMultiplayerModes(gameId, ctrl.signal)
      .then((list) => setModes(list))
      .catch((err) => {
        if (ctrl.signal.aborted) return;
        notifications.show({ color: "red", message: "Falha ao carregar os modos de multiplayer." });
        console.error(err);
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false);
      });
    return () => ctrl.abort();
  }, [gameId, reloadKey]);

  function backToList() {
    setView({ kind: "list" });
    setReloadKey((k) => k + 1);
  }

  async function confirmDelete() {
    if (!deleting) return;
    setRemoving(true);
    try {
      await api.deleteMultiplayerMode(gameId, deleting.modeId);
      notifications.show({
        color: "green",
        message: `Modo "${deleting.displayName || deleting.modeId}" removido.`,
      });
      setDeleting(null);
      setReloadKey((k) => k + 1);
    } catch (err) {
      const code = err instanceof HttpError ? err.code : undefined;
      notifications.show({
        color: "red",
        message: code ? `Falha ao remover (${code}).` : "Falha ao remover o modo.",
      });
    } finally {
      setRemoving(false);
    }
  }

  if (loading) return <Loader size="sm" />;

  if (view.kind !== "list") {
    return (
      <ModeEditor
        key={view.kind === "edit" ? view.mode.modeId : "new"}
        gameId={gameId}
        mode={view.kind === "edit" ? view.mode : null}
        lockDefault={view.kind === "create" && view.lockDefault}
        existingModeIds={modes.map((m) => m.modeId)}
        onSaved={backToList}
        onCancel={() => setView({ kind: "list" })}
      />
    );
  }

  if (modes.length === 0) {
    return (
      <Stack>
        <Alert color="blue" variant="light">
          Este jogo ainda não tem um modo de multiplayer. Configure o primeiro modo (ele será o
          modo <Code>default</Code> — o usado pelo matchmaking).
        </Alert>
        <Group>
          <Button onClick={() => setView({ kind: "create", lockDefault: true })}>
            Configurar multiplayer
          </Button>
        </Group>
      </Stack>
    );
  }

  return (
    <Stack>
      <Group justify="space-between">
        <Text size="sm" c="dimmed">
          Modos de multiplayer do jogo — cada modo é um perfil nomeado (arquétipo + parâmetros).
        </Text>
        <Button
          size="xs"
          leftSection={<IconPlus size={14} />}
          onClick={() => setView({ kind: "create", lockDefault: false })}
        >
          Novo modo
        </Button>
      </Group>

      <Table verticalSpacing="sm" highlightOnHover>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Modo</Table.Th>
            <Table.Th>ID</Table.Th>
            <Table.Th>Arquétipo</Table.Th>
            <Table.Th w={90} />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {modes.map((m) => {
            const meta = ARCHETYPE_META[m.archetype];
            return (
              <Table.Tr key={m.modeId}>
                <Table.Td>
                  <Text size="sm" fw={500}>
                    {m.displayName || m.modeId}
                  </Text>
                </Table.Td>
                <Table.Td>
                  <Code>{m.modeId}</Code>
                </Table.Td>
                <Table.Td>
                  <Badge variant="light" color={meta.color}>
                    {meta.label}
                  </Badge>
                </Table.Td>
                <Table.Td>
                  <Group gap={4} justify="flex-end" wrap="nowrap">
                    <ActionIcon
                      variant="subtle"
                      aria-label="Editar"
                      onClick={() => setView({ kind: "edit", mode: m })}
                    >
                      <IconPencil size={16} />
                    </ActionIcon>
                    {m.modeId !== DEFAULT_MODE_ID && (
                      <ActionIcon
                        variant="subtle"
                        color="red"
                        aria-label="Remover"
                        onClick={() => setDeleting(m)}
                      >
                        <IconTrash size={16} />
                      </ActionIcon>
                    )}
                  </Group>
                </Table.Td>
              </Table.Tr>
            );
          })}
        </Table.Tbody>
      </Table>

      <Modal
        opened={!!deleting}
        onClose={() => setDeleting(null)}
        title={`Remover o modo "${deleting ? deleting.displayName || deleting.modeId : ""}"?`}
      >
        <Stack>
          <Text size="sm">
            O modo <Code>{deleting?.modeId}</Code> será removido do cadastro do jogo. A ação é
            irreversível (histórico de partidas não é afetado).
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleting(null)} disabled={removing}>
              Cancelar
            </Button>
            <Button color="red" onClick={confirmDelete} loading={removing}>
              Remover
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
