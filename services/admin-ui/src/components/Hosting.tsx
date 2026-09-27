import { Alert, Badge, Group, Text, Tooltip } from "@mantine/core";
import { IconBroadcast, IconInfoCircle } from "@tabler/icons-react";
import type { GameSession } from "../lib/api";
import { formatDuration, truncate } from "../lib/format";
import { HEARTBEAT_META, heartbeatStatus, hostingOf, shortConnection } from "../lib/hosting";
import { useNow } from "../lib/useNow";

// Peças de hospedagem do console (admin-ui/091): quem hospeda o mundo
// (GameServer dedicado ou o jogo do host em p2p) e se ele está vivo
// (heartbeat, session/032). Usadas em Rooms e nas Sessões do jogo.

// HostingCell é a célula "Hospedagem": endereço do GameServer em dedicado;
// selo P2P + id de conexão do host em p2p.
export function HostingCell({ session: s }: { session: GameSession }) {
  const hosting = hostingOf(s);
  if (hosting === "p2p") {
    return (
      <Group gap={6} wrap="nowrap">
        <Badge size="xs" variant="light" color="grape">
          P2P
        </Badge>
        {s.hostConnection && (
          <Tooltip label={`Conexão do host: ${s.hostConnection}`} withArrow>
            <Text ff="monospace" size="sm">
              {shortConnection(s.hostConnection)}
            </Text>
          </Tooltip>
        )}
      </Group>
    );
  }
  if (s.serverAddress) {
    return (
      <Text ff="monospace" size="sm">
        {truncate(s.serverAddress)}
      </Text>
    );
  }
  // sessão fechada (servidor já devolvido) ou histórico sem topologia
  return (
    <Text size="sm" c="dimmed">
      —
    </Text>
  );
}

// HeartbeatBadge é o sinal de vida de um mundo aberto. Só faz sentido em
// host_world aberto — partidas e sessões fechadas não mostram nada.
export function HeartbeatBadge({ session: s }: { session: GameSession }) {
  if (s.origin !== "host_world" || s.state !== "open") return null;
  return <LiveHeartbeat session={s} />;
}

// LiveHeartbeat é separado para o relógio (useNow) só rodar em mundo aberto.
function LiveHeartbeat({ session: s }: { session: GameSession }) {
  const now = useNow(5_000);
  const status = heartbeatStatus(s.lastHeartbeatAt, now);
  const meta = HEARTBEAT_META[status];
  const age = s.lastHeartbeatAt ? ` · ${formatDuration(now - Date.parse(s.lastHeartbeatAt))}` : "";
  return (
    <Tooltip label={meta.help} withArrow multiline w={260}>
      <Badge
        size="xs"
        style={{ maxWidth: "none" }} // o texto é curto e informativo: não truncar
        variant={status === "none" ? "outline" : "dot"}
        color={meta.color}
        leftSection={status === "none" ? <IconBroadcast size={10} /> : undefined}
      >
        {meta.label}
        {age}
      </Badge>
    </Tooltip>
  );
}

// P2PTrustNote avisa, no detalhe de um mundo p2p, de onde vêm os eventos.
export function P2PTrustNote() {
  return (
    <Alert variant="light" color="grape" icon={<IconInfoCircle size={16} />} p="xs">
      <Text size="xs">
        Mundo P2P: o jogo do host hospeda e reporta os eventos deste mundo (world token). É a mesma confiança que os
        Souls dão ao host — conquistas e números deste mundo dependem da honestidade dele.
      </Text>
    </Alert>
  );
}

// closeEffect é o texto do modal "Encerrar" conforme quem hospeda.
export function closeEffect(s: GameSession): string {
  if (hostingOf(s) === "p2p") {
    return "O mundo sai da lista de invasão, o world token do host deixa de valer e o jogo do host recebe NotFound no próximo heartbeat.";
  }
  return "O game server é liberado e os jogadores conectados caem.";
}
