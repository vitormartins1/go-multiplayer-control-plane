import { useState } from "react";
import {
  Box,
  Button,
  Divider,
  Group,
  Modal,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useNavigate } from "react-router-dom";
import { api, HttpError, type Game } from "../lib/api";
import { GameCover } from "../components/GameCover";
import { FormSection } from "../components/FormSection";
import { GamePipeline } from "./GamePipeline";
import { FieldLabel, MultiplayerSettings } from "./MultiplayerSettings";

const MAX_DESCRIPTION_LEN = 500;

export function GameSettings({ game, onRenamed }: { game: Game; onRenamed: () => void }) {
  const navigate = useNavigate();
  const [name, setName] = useState(game.name);
  const [description, setDescription] = useState(game.description ?? "");
  const [saving, setSaving] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [confirmName, setConfirmName] = useState("");
  const [deleting, setDeleting] = useState(false);

  const trimmed = name.trim();
  const nameChanged = trimmed !== "" && trimmed !== game.name;
  const descriptionChanged = description !== (game.description ?? "");
  const descriptionTooLong = description.length > MAX_DESCRIPTION_LEN;
  const canSave = (nameChanged || descriptionChanged) && !descriptionTooLong && !saving;
  // Confirmação destrutiva (spec 038): digitar o nome exato do jogo.
  const canDelete = confirmName === game.name && !deleting;

  // Nome e descrição são independentes (2 endpoints) — dispara os dois juntos
  // quando ambos mudaram, em vez de sequencial (achado do /code-review: não
  // serializar chamadas que não dependem uma da outra).
  async function save() {
    setSaving(true);
    try {
      await Promise.all([
        nameChanged ? api.renameGame(game.id, trimmed) : Promise.resolve(),
        descriptionChanged ? api.setGameDescription(game.id, description) : Promise.resolve(),
      ]);
      notifications.show({ color: "nominal", message: "Jogo atualizado." });
      onRenamed();
    } catch {
      notifications.show({ color: "critical", message: "Falha ao salvar (dados inválidos ou serviço fora?)." });
    } finally {
      setSaving(false);
    }
  }

  async function removeGame() {
    setDeleting(true);
    try {
      await api.deleteGame(game.id);
      notifications.show({ color: "nominal", message: `"${game.name}" removido.` });
      navigate("/games");
    } catch (err) {
      if (err instanceof HttpError && err.status === 404) {
        // Já removido (outra aba/admin): o destino é o mesmo — a biblioteca.
        notifications.show({ color: "yellow", message: "Jogo não encontrado — já havia sido removido." });
        navigate("/games");
        return;
      }
      notifications.show({ color: "critical", message: "Falha ao remover (serviço fora?). Tente de novo." });
      setDeleting(false);
    }
  }

  // Capa quadrada à esquerda dos campos (revisão de layout, 2026-07-11 — 5ª
  // iteração: o banner de largura total ficou grande demais). Descrição +
  // "Criado em" preenchem o card com conteúdo real (spec 013); texto de ajuda
  // do Slug virou tooltip (FieldLabel), igual ao resto do formulário.
  return (
    <Stack>
      <Title order={4}>Identidade</Title>
      <FormSection label="Dados do jogo">
        <Group align="flex-start" gap="xl" wrap="wrap">
          <Box w={160} style={{ flexShrink: 0 }}>
            <GameCover game={game} withName={false} ratio={1} />
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
                label={<FieldLabel label="Slug" help="Chave estável — não muda." />}
                value={game.slug}
                readOnly
                disabled
              />
            </SimpleGrid>
            <Textarea
              label="Descrição"
              placeholder="Do que se trata este jogo? (opcional)"
              value={description}
              onChange={(e) => setDescription(e.currentTarget.value)}
              autosize
              minRows={2}
              maxRows={6}
              error={descriptionTooLong ? `Máximo de ${MAX_DESCRIPTION_LEN} caracteres.` : undefined}
              description={`${description.length}/${MAX_DESCRIPTION_LEN}`}
            />
            <Group justify="space-between" align="center">
              <Text size="xs" c="dimmed">
                Criado em {new Date(game.createdAt).toLocaleDateString("pt-BR")}
              </Text>
              <Button onClick={save} disabled={!canSave} loading={saving}>
                Salvar
              </Button>
            </Group>
          </Stack>
        </Group>
      </FormSection>

      <Divider my="md" />

      <Title order={4}>Multiplayer</Title>
      {/*
        Futuramente este container poderá listar múltiplos perfis (um por arquétipo),
        cada um numa ArchetypeCard independente — ex.: Queue Arena + Battle Royale.
        Por ora, apenas um perfil por jogo.
      */}
      <MultiplayerSettings gameId={game.id} />

      {/* Pipeline (spec 046, movido da aba Analytics — spec 053): mapa
          modos->envelope->consumers é sobre CONFIGURAÇÃO (o que este jogo
          tem cadastrado), não sobre métricas — faz mais sentido aqui, logo
          abaixo da listagem de modos, do que na aba de números. */}
      <Title order={5} mt="md">
        Pipeline de dados
      </Title>
      <GamePipeline gameId={game.id} />

      <Divider my="md" />

      <Title order={4}>Zona de perigo</Title>
      <FormSection label="Remover jogo">
        <Group justify="space-between" align="center" wrap="wrap">
          <Text size="sm" c="dimmed" style={{ flex: 1, minWidth: 260 }}>
            O jogo some do console e para de aceitar filas e sessões novas. Conquistas, saves,
            leaderboards e analytics ficam inacessíveis. Não há desfazer pelo console.
          </Text>
          <Button color="critical" variant="outline" onClick={() => setDeleteOpen(true)}>
            Remover jogo
          </Button>
        </Group>
      </FormSection>

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
            <Button color="critical" disabled={!canDelete} loading={deleting} onClick={removeGame}>
              Remover definitivamente
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
