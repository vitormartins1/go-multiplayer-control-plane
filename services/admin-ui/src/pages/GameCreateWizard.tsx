import { useState } from "react";
import {
  Alert,
  Badge,
  Box,
  Button,
  Code,
  Group,
  Modal,
  Stack,
  Stepper,
  Text,
  TextInput,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useNavigate } from "react-router-dom";
import { api, HttpError, type Game, type MultiplayerMode, type MultiplayerProfile } from "../lib/api";
import { slugify } from "../lib/slug";
import { GameCover } from "../components/GameCover";
import { ARCHETYPE_META } from "../lib/archetypeMeta";
import { ModeEditor, modeSummary } from "./MultiplayerSettings";

const SLUG_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

// Modo coletado no passo 2 — nada é salvo na API até o Confirmar (035).
type CollectedMode = { modeId: string; displayName: string; profile: MultiplayerProfile };

// previewGame monta um Game sintético para o preview da capa: o GameCover é
// client-side puro sobre a seed, então o preview reage ao digitar o slug.
function previewGame(slug: string, name: string): Game {
  return {
    id: "preview",
    slug,
    name: name || slug,
    createdAt: "",
    image: "",
    imageSource: "generated",
    generatedImageSeed: slug || "preview",
    generatedImageStyle: "shapes",
  } as Game;
}

// GameCreateWizard é a jornada de registro de jogo em 3 passos (035):
// Identidade (nome/slug + preview da capa) → Multiplayer opcional (ModeEditor
// em modo coleta) → Revisão (Confirmar executa POST /games e o PUT do modo).
export function GameCreateWizard({
  opened,
  onClose,
  onCreated,
}: {
  opened: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const nav = useNavigate();
  const [active, setActive] = useState(0);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  // slug segue o nome até o usuário editá-lo manualmente (padrão "touched").
  const [slugTouched, setSlugTouched] = useState(false);
  const [mode, setMode] = useState<CollectedMode | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const trimmedName = name.trim();
  const slugValid = SLUG_RE.test(slug);
  const step1Valid = trimmedName !== "" && slugValid;

  function reset() {
    setActive(0);
    setName("");
    setSlug("");
    setSlugTouched(false);
    setMode(null);
    setSubmitting(false);
    setSubmitError(null);
  }

  function close() {
    if (submitting) return;
    reset();
    onClose();
  }

  async function confirm() {
    setSubmitting(true);
    setSubmitError(null);
    try {
      const game = await api.createGame(slug, trimmedName);
      if (mode) {
        try {
          await api.setMultiplayerMode(game.id, mode.modeId, {
            ...mode.profile,
            ...(mode.displayName ? { displayName: mode.displayName } : {}),
          });
        } catch {
          // Jogo criado, modo não: sem rollback (jogo sem modo é válido) — o
          // usuário resolve em Configurações → Multiplayer.
          notifications.show({
            color: "yellow",
            message: `Jogo criado, mas o modo não foi salvo — configure em Configurações → Multiplayer.`,
          });
          reset();
          onCreated();
          nav(`/games/${game.id}`);
          return;
        }
      }
      notifications.show({ color: "green", message: `Jogo "${trimmedName}" registrado.` });
      reset();
      onCreated();
      nav(`/games/${game.id}`);
    } catch (e) {
      if (e instanceof HttpError && e.status === 409) {
        setSubmitError("Slug já existe — escolha outro no passo Identidade.");
        setActive(0);
      } else if (e instanceof HttpError && e.status === 400) {
        setSubmitError("Dados inválidos — revise o passo Identidade.");
        setActive(0);
      } else {
        setSubmitError("Falha ao registrar (serviço indisponível?). Tente novamente.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  const collectedAsMode: MultiplayerMode | null = mode
    ? ({ ...mode.profile, modeId: mode.modeId, displayName: mode.displayName } as MultiplayerMode)
    : null;

  return (
    <Modal opened={opened} onClose={close} title="Registrar jogo" size="xl">
      <Stepper active={active} onStepClick={setActive} allowNextStepsSelect={false} size="sm">
        <Stepper.Step label="Identidade" description="Nome, slug e capa">
          <Group align="flex-start" gap="xl" wrap="wrap" mt="md">
            <Stack gap="sm" style={{ flex: 1, minWidth: 260 }}>
              <TextInput
                label="Nome"
                placeholder="Meu Jogo"
                value={name}
                autoFocus
                onChange={(e) => {
                  setName(e.currentTarget.value);
                  if (!slugTouched) setSlug(slugify(e.currentTarget.value));
                }}
                maxLength={100}
              />
              <TextInput
                label="Slug"
                placeholder="meu-jogo"
                description="Chave estável do jogo — imutável após o registro. Também é a seed da capa gerada."
                value={slug}
                onChange={(e) => {
                  setSlugTouched(true);
                  setSlug(e.currentTarget.value);
                }}
                error={slug !== "" && !slugValid ? "slug minúsculo, sem espaços (a-z 0-9 -)" : undefined}
              />
              <Group justify="flex-end" mt="sm">
                <Button onClick={() => setActive(1)} disabled={!step1Valid}>
                  Continuar
                </Button>
              </Group>
            </Stack>
            <Box w={150} style={{ flexShrink: 0 }}>
              <GameCover game={previewGame(slug, trimmedName)} />
              <Text size="xs" c="dimmed" ta="center" mt={6}>
                Preview da capa gerada
              </Text>
            </Box>
          </Group>
        </Stepper.Step>

        <Stepper.Step label="Multiplayer" description="Primeiro modo (opcional)">
          <Stack mt="md">
            <Text size="sm" c="dimmed">
              Configure o primeiro modo de multiplayer (será o modo <Code>default</Code>, usado
              pelo matchmaking) — ou pule e configure depois em Configurações. Nada é salvo até a
              confirmação final.
            </Text>
            <ModeEditor
              key={mode ? "collected" : "new"}
              mode={collectedAsMode}
              lockDefault
              existingModeIds={[]}
              onSubmit={async (modeId, displayName, profile) => {
                setMode({ modeId, displayName, profile });
              }}
              onSaved={() => setActive(2)}
              onCancel={() => {
                setMode(null);
                setActive(2);
              }}
              submitLabel="Usar este modo"
              cancelLabel="Pular multiplayer"
            />
          </Stack>
        </Stepper.Step>

        <Stepper.Step label="Revisão" description="Confirmar registro">
          <Group align="flex-start" gap="xl" wrap="wrap" mt="md">
            <Box w={150} style={{ flexShrink: 0 }}>
              <GameCover game={previewGame(slug, trimmedName)} />
            </Box>
            <Stack gap="xs" style={{ flex: 1, minWidth: 260 }}>
              <Text fw={600} size="lg">
                {trimmedName}
              </Text>
              <Group gap="xs">
                <Text size="sm" c="dimmed">
                  slug:
                </Text>
                <Code>{slug}</Code>
              </Group>
              {collectedAsMode ? (
                <Group gap="xs">
                  <Badge variant="light" color={ARCHETYPE_META[collectedAsMode.archetype].color}>
                    {collectedAsMode.modeId}
                  </Badge>
                  <Text size="sm" c="dimmed">
                    {modeSummary(collectedAsMode)}
                  </Text>
                </Group>
              ) : (
                <Badge variant="light" color="gray" w="fit-content">
                  sem multiplayer por ora
                </Badge>
              )}
              {submitError && (
                <Alert color="red" variant="light">
                  {submitError}
                </Alert>
              )}
              <Group mt="sm">
                <Button onClick={confirm} loading={submitting}>
                  Confirmar registro
                </Button>
                <Button variant="default" onClick={() => setActive(1)} disabled={submitting}>
                  Voltar
                </Button>
              </Group>
            </Stack>
          </Group>
        </Stepper.Step>
      </Stepper>
    </Modal>
  );
}
