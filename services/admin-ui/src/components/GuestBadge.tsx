import { Badge } from "@mantine/core";
import { isGuest } from "./PlayerAvatar";

// GuestBadge marca jogadores de login anônimo (024): renderiza um badge
// discreto quando o playerId é de guest ("guest-<uuid>"), nada caso contrário.
export function GuestBadge({ playerId, size = "xs" }: { playerId: string; size?: string }) {
  if (!isGuest(playerId)) return null;
  return (
    <Badge size={size} variant="light" color="gray">
      guest
    </Badge>
  );
}
