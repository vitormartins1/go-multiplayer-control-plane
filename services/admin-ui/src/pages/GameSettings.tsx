import { useState } from "react";
import { Box, Button, Divider, Fieldset, Grid, Group, SimpleGrid, Stack, Text, TextInput, Title } from "@mantine/core";
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

  // Capa em coluna própria à direita (ajuste da 026): tamanho dinâmico (1/3 da
  // largura em md+), sticky ao rolar os forms longos; em telas pequenas cai
  // para o final (ordem natural do DOM: conteúdo antes, capa depois).
  return (
    <Grid maw={1100} gutter="xl">
      <Grid.Col span={{ base: 12, md: 8 }}>
        <Stack>
          <Title order={4}>Identidade</Title>
          <Fieldset legend="Dados do jogo">
            <Stack gap="sm">
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
      </Grid.Col>

      <Grid.Col span={{ base: 12, md: 4 }}>
        <Box style={{ position: "sticky", top: 72 }}>
          <GameCover game={game} />
          <Text size="xs" c="dimmed" ta="center" mt={6}>
            Capa gerada — determinística pela seed do jogo.
          </Text>
        </Box>
      </Grid.Col>
    </Grid>
  );
}
