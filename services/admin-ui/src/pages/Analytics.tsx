import { Stack, Title } from "@mantine/core";
import { ComingSoon } from "../components/ComingSoon";

export function Analytics() {
  return (
    <Stack>
      <Title order={3}>Analytics</Title>
      <ComingSoon what="Game analytics" />
    </Stack>
  );
}
