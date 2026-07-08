import { useState } from "react";
import { Box, Button, Divider, Fieldset, Group, Stack, Text, TextInput, Title } from "@mantine/core";
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

  // Capa DENTRO de Dados do jogo (036, 3ª iteração do layout): a capa é conteúdo
  // de identidade, não merece coluna própria — a página volta a ser coluna única
  // e o Multiplayer usa a largura toda. Sem faixa de nome na capa (o nome está
  // no input ao lado). Em tela estreita o Group quebra e a capa fica acima.
  return (
    <Stack maw={900}>
      <Title order={4}>Identidade</Title>
      <Fieldset legend="Dados do jogo">
        <Group align="flex-start" gap="xl" wrap="wrap">
          <Box w={150} style={{ flexShrink: 0 }}>
            <GameCover game={game} withName={false} />
            <Text size="xs" c="dimmed" ta="center" mt={6}>
              Capa gerada — determinística pela seed do jogo.
            </Text>
          </Box>
          <Stack gap="sm" style={{ flex: 1, minWidth: 260 }}>
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
