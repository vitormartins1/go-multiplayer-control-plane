import { Text, Title } from "@mantine/core";
import type { Icon } from "@tabler/icons-react";
import "./ChannelStrip.css";

// ChannelStrip é o cabeçalho de seção "canal de telemetria" (nasceu na aba
// Analytics, spec 071): badge do ícone, régua de timebase e leitura mono do
// total do canal, no lugar de um título solto ou de um Divider com label.
// Compartilhado (spec 074) — qualquer lugar que agrupe cards/tabelas por
// categoria com um total que vale a pena destacar pode reusar.
export function ChannelStrip({
  icon: SectionIcon,
  label,
  readout,
}: {
  icon: Icon;
  label: string;
  readout?: string;
}) {
  return (
    <div className="channel-strip">
      <div className="channel-strip-badge">
        <SectionIcon size={16} stroke={1.75} />
      </div>
      <Title order={4} className="channel-strip-label">
        {label}
      </Title>
      <div className="channel-strip-rule" aria-hidden />
      {readout && (
        <Text ff="monospace" size="xs" c="dimmed" className="channel-strip-readout">
          {readout}
        </Text>
      )}
    </div>
  );
}
