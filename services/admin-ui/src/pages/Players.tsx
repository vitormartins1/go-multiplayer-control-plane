import { Alert, Button, Group, Loader, Stack, Table, Text, TextInput, Title } from "@mantine/core";
import { useNavigate } from "react-router-dom";
import { useState } from "react";
import { IconRefresh, IconSearch } from "@tabler/icons-react";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { PlayerAvatar } from "../components/PlayerAvatar";

const PAGE = 50;

// Players é a base GLOBAL de usuários (identity) — independente de jogo. Abrir um
// jogador leva ao detalhe, onde se escolhe entre os jogos DELE. Para ver "quem
// joga o jogo X", use Games → [jogo] → aba Players.
export function Players() {
  const nav = useNavigate();
  const [offset, setOffset] = useState(0);
  const [search, setSearch] = useState("");
  const { data, loading, error, reload } = useAsync((s) => api.players(PAGE, offset, s), [offset]);

  const filtered = data?.players.filter((p) =>
    search.trim() === "" || p.handle.toLowerCase().includes(search.toLowerCase()),
  ) ?? [];

  const total = data?.total ?? 0;
  const canPrev = offset > 0;
  const canNext = offset + PAGE < total;

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>
          Players {data ? <Text span c="dimmed">· {total}</Text> : null}
        </Title>
        <Button variant="default" leftSection={<IconRefresh size={16} />} onClick={reload}>
          Atualizar
        </Button>
      </Group>

      <TextInput
        placeholder="Filtrar por handle…"
        leftSection={<IconSearch size={16} />}
        value={search}
        onChange={(e) => setSearch(e.currentTarget.value)}
        w={280}
      />

      {loading && !data && <Loader />}
      {!!error && <Alert color="critical">identity indisponível.</Alert>}
      {data && filtered.length === 0 && (
        <Text c="dimmed">{search ? "Nenhum resultado para este filtro." : "Nenhum usuário cadastrado."}</Text>
      )}
      {data && filtered.length > 0 && (
        <>
          <Table striped highlightOnHover withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Handle</Table.Th>
                <Table.Th>Id</Table.Th>
                <Table.Th>Criado</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {filtered.map((p) => (
                <Table.Tr key={p.id} style={{ cursor: "pointer" }} onClick={() => nav(`/players/${p.id}`)}>
                  <Table.Td>
                    <Group gap="sm" wrap="nowrap">
                      <PlayerAvatar seed={p.id} />
                      <span>{p.handle}</span>
                    </Group>
                  </Table.Td>
                  <Table.Td c="dimmed" ff="monospace">{p.id.slice(0, 8)}…</Table.Td>
                  <Table.Td>{new Date(p.createdAt).toLocaleDateString("pt-BR")}</Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
          {!search && (
            <Group>
              <Button variant="default" size="xs" disabled={!canPrev} onClick={() => setOffset(Math.max(0, offset - PAGE))}>
                ← Anterior
              </Button>
              <Text c="dimmed" size="sm">
                {Math.min(offset + 1, total)}–{Math.min(offset + PAGE, total)} de {total}
              </Text>
              <Button variant="default" size="xs" disabled={!canNext} onClick={() => setOffset(offset + PAGE)}>
                Próxima →
              </Button>
            </Group>
          )}
        </>
      )}
    </Stack>
  );
}
