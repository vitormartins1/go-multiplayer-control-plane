import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Group,
  Loader,
  NumberInput,
  Select,
  SegmentedControl,
  Stack,
  Switch,
  TagsInput,
  Text,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  api,
  HttpError,
  type MultiplayerProfile,
  type QueueArenaParams,
} from "../lib/api";

// Regra de identificador de stat (espelha a validação do game-registry/006).
const STAT_RE = /^[a-z][a-z0-9_]*$/;

// Estado default de um perfil "não configurado" (queue_arena/dedicated, ffa,
// first_to_score, single). O servidor é a verdade — isto só semeia o form.
const DEFAULTS: QueueArenaParams = {
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

// buildProfile monta o payload zerando os campos incoerentes com os
// discriminadores — o backend rejeita perfis ambíguos (ex.: teamCount num ffa).
function buildProfile(topology: "dedicated" | "p2p", p: QueueArenaParams): MultiplayerProfile {
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

// MultiplayerSettings edita o perfil de multiplayer do jogo (ADR-0023, spec 015).
// Campos condicionais espelham a validação do game-registry/006; o servidor é a
// verdade — a validação client-side só evita submissões obviamente inválidas.
export function MultiplayerSettings({ gameId }: { gameId: string }) {
  const [loading, setLoading] = useState(true);
  const [configured, setConfigured] = useState(false);
  const [topology, setTopology] = useState<"dedicated" | "p2p">("dedicated");
  const [p, setP] = useState<QueueArenaParams>(DEFAULTS);
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
          setP({ ...DEFAULTS, ...profile.params });
        } else {
          setConfigured(false);
          setTopology("dedicated");
          setP(DEFAULTS);
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

  // Helper p/ atualizar um campo dos params.
  function set<K extends keyof QueueArenaParams>(key: K, value: QueueArenaParams[K]) {
    setP((prev) => ({ ...prev, [key]: value }));
  }

  const invalidStats = p.scoringStats.filter((s) => !STAT_RE.test(s));
  const canSave = invalidStats.length === 0 && !saving;

  async function save() {
    setSaving(true);
    try {
      await api.setMultiplayerProfile(gameId, buildProfile(topology, p));
      setConfigured(true);
      notifications.show({ color: "green", message: "Perfil de multiplayer salvo." });
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

  if (loading) return <Loader size="sm" />;

  return (
    <Stack maw={480}>
      {!configured && (
        <Alert color="blue" variant="light">
          Este jogo ainda não tem um perfil de multiplayer. Defina abaixo e salve para configurá-lo.
        </Alert>
      )}

      <Select
        label="Arquétipo"
        value="queue_arena"
        data={[{ value: "queue_arena", label: "Queue Arena (partida simétrica NvN)" }]}
        description="Único arquétipo suportado por ora."
        readOnly
      />

      <div>
        <Text size="sm" fw={500} mb={4}>
          Estrutura
        </Text>
        <SegmentedControl
          value={p.structure}
          onChange={(v) => set("structure", v as QueueArenaParams["structure"])}
          data={[
            { value: "ffa", label: "FFA (todos contra todos)" },
            { value: "teams", label: "Times" },
          ]}
        />
      </div>

      {p.structure === "ffa" ? (
        <NumberInput
          label="Jogadores na partida"
          min={2}
          value={p.playerCount}
          onChange={(v) => set("playerCount", typeof v === "number" ? v : 0)}
        />
      ) : (
        <Group grow>
          <NumberInput
            label="Nº de times"
            min={2}
            value={p.teamCount}
            onChange={(v) => set("teamCount", typeof v === "number" ? v : 0)}
          />
          <NumberInput
            label="Jogadores por time"
            min={1}
            value={p.teamSize}
            onChange={(v) => set("teamSize", typeof v === "number" ? v : 0)}
          />
        </Group>
      )}

      <Select
        label="Condição de vitória"
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
          label="Pontuação-alvo"
          min={1}
          value={p.scoreTarget}
          onChange={(v) => set("scoreTarget", typeof v === "number" ? v : 0)}
        />
      )}
      {p.winCondition === "highest_at_time" && (
        <NumberInput
          label="Duração (segundos)"
          min={1}
          value={p.durationSeconds}
          onChange={(v) => set("durationSeconds", typeof v === "number" ? v : 0)}
        />
      )}

      <div>
        <Text size="sm" fw={500} mb={4}>
          Formato da partida
        </Text>
        <SegmentedControl
          value={p.matchStructure}
          onChange={(v) => set("matchStructure", v as QueueArenaParams["matchStructure"])}
          data={[
            { value: "single", label: "Partida única" },
            { value: "best_of_rounds", label: "Melhor de N rounds" },
          ]}
        />
      </div>

      {p.matchStructure === "best_of_rounds" && (
        <NumberInput
          label="Rounds para vencer"
          min={1}
          value={p.roundsToWin}
          onChange={(v) => set("roundsToWin", typeof v === "number" ? v : 0)}
        />
      )}

      <Switch
        label="Permitir empate"
        checked={p.allowDraw}
        onChange={(e) => set("allowDraw", e.currentTarget.checked)}
      />

      <TagsInput
        label="Stats de pontuação"
        description="Métricas que o resultado carrega (ex.: kills, assists). Minúsculas, começa com letra."
        value={p.scoringStats}
        onChange={(v) => set("scoringStats", v)}
        error={
          invalidStats.length > 0
            ? `Stat inválido: ${invalidStats.join(", ")} (use ^[a-z][a-z0-9_]*$)`
            : undefined
        }
        clearable
      />

      <Tooltip label="p2p é reservado (ADR-0023) e ainda não implementado — o servidor rejeita.">
        <Select
          label="Topologia"
          value={topology}
          onChange={(v) => v && setTopology(v as "dedicated" | "p2p")}
          allowDeselect={false}
          data={[
            { value: "dedicated", label: "Dedicated Server" },
            { value: "p2p", label: "P2P (reservado)" },
          ]}
        />
      </Tooltip>

      <Group>
        <Button onClick={save} disabled={!canSave} loading={saving}>
          {configured ? "Salvar" : "Configurar multiplayer"}
        </Button>
      </Group>
    </Stack>
  );
}
