import { Group, Menu, Text, UnstyledButton } from "@mantine/core";
import { IconChevronDown } from "@tabler/icons-react";
import type { Game } from "../lib/api";
import { GameCover } from "./GameCover";

const MAX_VISIBLE = 5;

// chipStyle: identidade visual do chip; o selecionado ganha borda/fundo na cor
// primária para leitura imediata de "qual jogo está regendo a página".
function chipStyle(selected: boolean): React.CSSProperties {
  return {
    display: "flex",
    alignItems: "center",
    gap: 8,
    padding: "4px 10px 4px 4px",
    borderRadius: 8,
    border: selected
      ? "1px solid var(--mantine-color-blue-5)"
      : "1px solid var(--mantine-color-default-border)",
    backgroundColor: selected ? "var(--mantine-color-blue-light)" : "transparent",
  };
}

function GameChip({ game, selected, onClick }: { game: Game; selected: boolean; onClick: () => void }) {
  return (
    <UnstyledButton onClick={onClick} style={chipStyle(selected)} aria-pressed={selected}>
      <GameCover game={game} variant="mini" />
      <Text size="sm" fw={selected ? 600 : 400} truncate maw={110}>
        {game.name}
      </Text>
    </UnstyledButton>
  );
}

// GameChipsSelector é o seletor de jogo por capas do PlayerDetail (spec 040):
// substitui o dropdown por chips com a mini-capa + nome. Até MAX_VISIBLE chips;
// o excedente vai para um menu "+N". O selecionado nunca fica escondido: quando
// está no overflow, ele toma o lugar do último chip visível.
export function GameChipsSelector({
  games,
  value,
  onChange,
}: {
  games: Game[];
  value: string | null;
  onChange: (id: string) => void;
}) {
  const visible = games.slice(0, MAX_VISIBLE);
  const selectedIdx = games.findIndex((g) => g.id === value);
  if (selectedIdx >= MAX_VISIBLE) visible[MAX_VISIBLE - 1] = games[selectedIdx];
  const overflow = games.filter((g) => !visible.some((v) => v.id === g.id));

  return (
    <Group gap="xs" wrap="wrap" justify="flex-end">
      {visible.map((g) => (
        <GameChip key={g.id} game={g} selected={g.id === value} onClick={() => onChange(g.id)} />
      ))}
      {overflow.length > 0 && (
        <Menu position="bottom-end" withinPortal>
          <Menu.Target>
            <UnstyledButton style={chipStyle(false)}>
              <Text size="sm" c="dimmed">
                +{overflow.length}
              </Text>
              <IconChevronDown size={14} style={{ color: "var(--mantine-color-dimmed)" }} />
            </UnstyledButton>
          </Menu.Target>
          <Menu.Dropdown>
            {overflow.map((g) => (
              <Menu.Item
                key={g.id}
                leftSection={<GameCover game={g} variant="mini" />}
                onClick={() => onChange(g.id)}
              >
                <Text size="sm" truncate maw={180}>
                  {g.name}
                </Text>
              </Menu.Item>
            ))}
          </Menu.Dropdown>
        </Menu>
      )}
    </Group>
  );
}
