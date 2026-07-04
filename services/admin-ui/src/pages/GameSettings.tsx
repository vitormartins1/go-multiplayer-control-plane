import { useState } from "react";
import { Button, Group, Stack, Text, TextInput } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { api, type Game } from "../lib/api";

// GameSettings é a aba Configurações do jogo (spec 012): renomear (único write
// de config por ora; delete adiado). O slug é a chave estável — read-only.
export function GameSettings({ game, onRenamed }: { game: Game; onRenamed: () => void }) {
  const [name, setName] = useState(game.name);
  const [saving, setSaving] = useState(false);

  const trimmed = name.trim();
  const canSave = trimmed !== "" && trimmed !== game.name && !saving;

  async function save() {
    setSaving(true);
    try {
      await api.renameGame(game.id, trimmed);
      notifications.show({ color: "green", message: "Jogo renomeado." });
      onRenamed();
    } catch {
      notifications.show({ color: "red", message: "Falha ao renomear (nome inválido ou serviço fora?)." });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Stack maw={420}>
      <TextInput
        label="Nome"
        value={name}
        onChange={(e) => setName(e.currentTarget.value)}
        maxLength={100}
      />
      <TextInput label="Slug" value={game.slug} readOnly disabled description="Chave estável — não muda." />
      <Group>
        <Button onClick={save} disabled={!canSave} loading={saving}>
          Salvar
        </Button>
      </Group>
      <Text size="xs" c="dimmed">
        Remover jogo ainda não está disponível (envolve apagar dados em vários serviços).
      </Text>
    </Stack>
  );
}
