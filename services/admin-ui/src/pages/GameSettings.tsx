import { useState } from "react";
import {
  Box,
  Button,
  Divider,
  Fieldset,
  Group,
  Modal,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useNavigate } from "react-router-dom";
import { api, HttpError, type Game } from "../lib/api";
import { GameCover } from "../components/GameCover";
import { MultiplayerSettings } from "./MultiplayerSettings";

export function GameSettings({ game, onRenamed }: { game: Game; onRenamed: () => void }) {
  const navigate = useNavigate();
  const [name, setName] = useState(game.name);
  const [saving, setSaving] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [confirmName, setConfirmName] = useState("");
  const [deleting, setDeleting] = useState(false);

  const trimmed = name.trim();
  const canSave = trimmed !== "" && trimmed !== game.name && !saving;
  // Confirmação destrutiva (spec 038): digitar o nome exato do jogo.
  const canDelete = confirmName === game.name && !deleting;

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

  async function removeGame() {
    setDeleting(true);
    try {
      await api.deleteGame(game.id);
      notifications.show({ color: "green", message: `"${game.name}" removido.` });
      navigate("/games");
    } catch (err) {
      if (err instanceof HttpError && err.status === 404) {
        // Já removido (outra aba/admin): o destino é o mesmo — a biblioteca.
        notifications.show({ color: "yellow", message: "Jogo não encontrado — já havia sido removido." });
        navigate("/games");
        return;
      }
      notifications.show({ color: "red", message: "Falha ao remover (serviço fora?). Tente de novo." });
      setDeleting(false);
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

      <Divider my="md" />

      <Title order={4}>Zona de perigo</Title>
      <Fieldset legend="Remover jogo">
        <Group justify="space-between" align="center" wrap="wrap">
          <Text size="sm" c="dimmed" style={{ flex: 1, minWidth: 260 }}>
            O jogo some do console e para de aceitar filas e sessões novas. Conquistas, saves,
            leaderboards e analytics ficam inacessíveis. Não há desfazer pelo console.
          </Text>
          <Button color="red" variant="outline" onClick={() => setDeleteOpen(true)}>
            Remover jogo
          </Button>
        </Group>
      </Fieldset>

      <Modal
        opened={deleteOpen}
        onClose={() => {
          setDeleteOpen(false);
          setConfirmName("");
        }}
        title={`Remover "${game.name}"`}
      >
        <Stack gap="sm">
          <Text size="sm">
            Esta ação remove o jogo do console e bloqueia novas filas e sessões. Sessões em
            andamento terminam naturalmente. Conquistas, saves, leaderboards e analytics ficam
            inacessíveis. <strong>Não há desfazer pelo console.</strong>
          </Text>
          <TextInput
            label={`Digite "${game.name}" para confirmar`}
            value={confirmName}
            onChange={(e) => setConfirmName(e.currentTarget.value)}
            data-autofocus
          />
          <Group justify="flex-end">
            <Button
              variant="default"
              onClick={() => {
                setDeleteOpen(false);
                setConfirmName("");
              }}
            >
              Cancelar
            </Button>
            <Button color="red" disabled={!canDelete} loading={deleting} onClick={removeGame}>
              Remover definitivamente
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
