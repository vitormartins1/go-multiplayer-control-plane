import { Badge } from "@mantine/core";
import { fleetLocationColor, fleetLocationLabel, type FleetLocation } from "../lib/fleetLocation";

export function FleetLocationBadge({ location }: { location: FleetLocation }) {
  return (
    <Badge color={fleetLocationColor(location)} variant="light" size="sm">
      {fleetLocationLabel(location)}
    </Badge>
  );
}
