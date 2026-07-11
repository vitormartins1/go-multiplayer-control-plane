import { useState } from "react";
import { Alert, Button, Drawer, Group, SimpleGrid, Stack, TextInput } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useNavigate } from "react-router-dom";
import { api, HttpError, type Game } from "../lib/api";
import { slugify } from "../lib/slug";
import { GameCover } from "../components/GameCover";
import { FormSection } from "../components/FormSection";
import {
  FieldLabel,
  ModeIdentityFields,
  ModeProfileFields,
  SwitchWithHelp,
  useModeIdentityState,
  useModeProfileState,
} from "./MultiplayerSettings";

const SLUG_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

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

// GameCreateWizard é o drawer de registro de jogo (revisão de layout,
// 2026-07-11 — substituiu o Modal com Stepper de 3 passos): identidade e
// multiplayer na mesma tela, divididos por seção, um único envio no fim. O
// formulário (GameCreateForm) é remontado via `key` a cada abertura — reseta
// o estado sem precisar de um reset() manual espalhado pelos campos.
export function GameCreateWizard({
  opened,
  onClose,
  onCreated,
}: {
  opened: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [resetKey, setResetKey] = useState(0);

  function handleClose() {
    onClose();
    setResetKey((k) => k + 1);
  }

  return (
    <Drawer opened={opened} onClose={handleClose} title="Registrar jogo" position="right" size={620}>
      <GameCreateForm key={resetKey} onClose={handleClose} onCreated={onCreated} />
    </Drawer>
  );
}

function GameCreateForm({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const nav = useNavigate();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  // slug segue o nome até o usuário editá-lo manualmente (padrão "touched").
  const [slugTouched, setSlugTouched] = useState(false);
  // Switch desmarcado por padrão: o form de multiplayer já vem aberto pra
  // preencher agora; marcar "configurar depois" esconde os campos e pula.
  const [skipMultiplayer, setSkipMultiplayer] = useState(false);
  // Primeiro modo do jogo (lockDefault=true, sem outros IDs pra colidir) —
  // mesma identidade (ID travado em "default" + nome de exibição livre) do
  // bootstrap "Configurar multiplayer" em Configurações.
  const modeIdentity = useModeIdentityState(null, true, []);
  const profile = useModeProfileState(null);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const trimmedName = name.trim();
  const slugValid = SLUG_RE.test(slug);
  const identityValid = trimmedName !== "" && slugValid;
  const canSubmit =
    identityValid && (skipMultiplayer || (modeIdentity.valid && profile.valid)) && !submitting;

  async function confirm() {
    setSubmitting(true);
    setSubmitError(null);
    try {
      const game = await api.createGame(slug, trimmedName);
      if (!skipMultiplayer) {
        try {
          const modeDisplayName = modeIdentity.displayName.trim();
          await api.setMultiplayerMode(game.id, modeIdentity.modeId, {
            ...profile.buildProfile(),
            ...(modeDisplayName ? { displayName: modeDisplayName } : {}),
          });
        } catch {
          // Jogo criado, modo não: sem rollback (jogo sem modo é válido) — o
          // usuário resolve em Configurações → Multiplayer.
          notifications.show({
            color: "yellow",
            message: "Jogo criado, mas o modo não foi salvo — configure em Configurações → Multiplayer.",
          });
          onCreated();
          nav(`/games/${game.id}`);
          return;
        }
      }
      notifications.show({ color: "nominal", message: `Jogo "${trimmedName}" registrado.` });
      onCreated();
      nav(`/games/${game.id}`);
    } catch (e) {
      if (e instanceof HttpError && e.status === 409) {
        setSubmitError("Slug já existe — escolha outro.");
      } else if (e instanceof HttpError && e.status === 400) {
        setSubmitError("Dados inválidos — revise os campos.");
      } else {
        setSubmitError("Falha ao registrar (serviço indisponível?). Tente novamente.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Stack>
      {/* capa no topo, formato banner largo — a capa é gerada proceduralmente
          e aceita qualquer proporção (036/049); sem faixa de nome (os campos
          de Nome/Slug já estão logo abaixo). */}
      <GameCover game={previewGame(slug, trimmedName)} withName={false} ratio={4} />

      <FormSection label="Identidade">
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
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
            label={
              <FieldLabel
                label="Slug"
                help="Chave estável do jogo — imutável após o registro. Também é a seed da capa gerada."
              />
            }
            placeholder="meu-jogo"
            value={slug}
            onChange={(e) => {
              setSlugTouched(true);
              setSlug(e.currentTarget.value);
            }}
            error={slug !== "" && !slugValid ? "slug minúsculo, sem espaços (a-z 0-9 -)" : undefined}
          />
        </SimpleGrid>
      </FormSection>

      <FormSection label="Multiplayer">
        <SwitchWithHelp
          label="Configurar depois"
          help='Deixa "Configurar depois" marcado para registrar o jogo sem modo de multiplayer — configure quando quiser em Configurações → Multiplayer.'
          checked={skipMultiplayer}
          onChange={setSkipMultiplayer}
        />
      </FormSection>

      {/* Sem "Configurar depois": as seções seguem soltas (mesmo nível dos
          cartões acima), igual ao editor de modo em Configurações — nada de
          cartão dentro de cartão. */}
      {!skipMultiplayer && (
        <>
          <FormSection label="Identidade do modo">
            <ModeIdentityFields state={modeIdentity} />
          </FormSection>
          <ModeProfileFields state={profile} />
        </>
      )}

      {submitError && (
        <Alert color="critical" variant="light">
          {submitError}
        </Alert>
      )}

      <Group>
        <Button onClick={confirm} disabled={!canSubmit} loading={submitting}>
          Registrar jogo
        </Button>
        <Button variant="default" onClick={onClose} disabled={submitting}>
          Cancelar
        </Button>
      </Group>
    </Stack>
  );
}
