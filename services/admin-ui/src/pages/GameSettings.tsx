import { useState } from "react";
import { Box, Button, Divider, Fieldset, Group, SimpleGrid, Stack, Text, TextInput, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { api, type Game } from "../lib/api";
import { GameCover } from "../components/GameCover";
import { MultiplayerSettings } from "./MultiplayerSettings";

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
    <Stack maw={960}>
      <Title order={4}>Identidade</Title>
      <Fieldset legend="Dados do jogo">
        {/* Capa gerada ao lado dos campos (026) — a "identidade" visual do jogo. */}
        <Group align="flex-start" gap="lg">
          <Box w={110} style={{ flexShrink: 0 }}>
            <GameCover game={game} />
          </Box>
          <Stack gap="sm" style={{ flex: 1, minWidth: 260 }}>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
              <TextInput
                label="Nome"
                value={name}
                onChange={(e) => setName(e.currentTarget.value)}
                maxLength={100}
              />
              <TextInput
                label="Slug"
                value={game.slug}
                readOnly
                disabled
                description="Chave estável — não muda."
              />
            </SimpleGrid>
            <Group>
              <Button onClick={save} disabled={!canSave} loading={saving}>
                Salvar
              </Button>
            </Group>
            <Text size="xs" c="dimmed">
              Remover jogo ainda não está disponível (envolve apagar dados em vários serviços).
            </Text>
          </Stack>
        </Group>
      </Fieldset>

      <Divider my="md" />

      <Title order={4}>Multiplayer</Title>
      {/*
        Futuramente este container poderá listar múltiplos perfis (um por arquétipo),
        cada um numa ArchetypeCard independente — ex.: Queue Arena + Battle Royale.
        Por ora, apenas um perfil por jogo.
      */}
      <MultiplayerSettings gameId={game.id} />
    </Stack>
  );
}
