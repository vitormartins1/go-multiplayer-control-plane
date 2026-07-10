import { Alert, Loader } from "@mantine/core";
import { useEffect, useState } from "react";
import { api, type MultiplayerProfile } from "../lib/api";
import { gameArchetypes, filterRulesForGame } from "../lib/eventRules";
import { useAsync } from "../lib/useAsync";
import { GameEventRulesCanvas } from "./GameEventRulesCanvas";

// Fallback só pro caso raro de o jogo ainda não ter NENHUM modo/perfil
// multiplayer cadastrado (dropdown não pode ficar vazio) — usado só pro
// picker de arquétipo do form de criação in-canvas (archetypeOptions).
const ALL_ARCHETYPES: MultiplayerProfile["archetype"][] = [
  "host_world_dropin",
  "queue_arena",
  "battle_royale",
  "coop_mission",
];

// GameEventRules (spec 058): shell de DADO só — fetch de rules/modes/
// catalog + sincronização de pendingRuleFocus (navegação vinda da aba
// Achievements). Todo o layout (lista, toolbar, texto explicativo) migrou
// pra dentro de GameEventRulesCanvas como painel flutuante sobre o canvas —
// antes disso essa página tinha sua própria coluna de lista + modal de
// delete + toggleSuppress, duplicando o que o canvas já precisa ter pro
// NodeDetails; agora só existe uma cópia de cada, dentro do canvas.
export function GameEventRules({
  gameId,
  pendingRuleFocus,
  onConsumePendingRuleFocus,
}: {
  gameId: string;
  // pendingRuleFocus (spec 057): navegação vinda da aba Achievements com uma
  // regra específica pra focar (e opcionalmente editar). Consumido uma vez
  // (onConsumePendingRuleFocus) pra não re-disparar em renders seguintes.
  pendingRuleFocus?: { ruleId: string; edit: boolean } | null;
  onConsumePendingRuleFocus?: () => void;
}) {
  const { data: rules, loading, error, reload } = useAsync((s) => api.gameEventRules(gameId, s), [gameId]);
  // modos/perfis do jogo — a fonte de verdade de quais arquétipos ele
  // realmente usa (evita listar/oferecer arquétipos que o jogo não tem).
  const { data: modes } = useAsync((s) => api.listMultiplayerModes(gameId, s), [gameId]);
  // catálogo de achievements — 4ª camada do canvas (quem escuta cada type).
  const { data: catalog, reload: reloadCatalog } = useAsync((s) => api.gameCatalog(gameId, s), [gameId]);
  // Atualizar recarrega as DUAS fontes do canvas — um achievement recém-criado
  // na aba Achievements precisa aparecer na camada 4 sem trocar de aba.
  function reloadAll() {
    reload();
    reloadCatalog();
  }
  const archetypes = gameArchetypes(modes ?? []);
  // Sem nenhum modo cadastrado ainda: cai pra lista fixa (dropdown não pode
  // ficar vazio) — ver filterRulesForGame pro mesmo raciocínio na listagem.
  const archetypeOptions = archetypes.size > 0 ? [...archetypes] : ALL_ARCHETYPES;
  const visibleRules = filterRulesForGame(rules ?? [], archetypes);

  const [focusRuleId, setFocusRuleId] = useState<string | null>(null);

  // Consome pendingRuleFocus só quando as regras já carregaram (senão
  // focaria um id que ainda não existe na lista) — limpa no pai logo depois,
  // então autoEditRuleId (repassado ao canvas) só fica setado 1-2 renders.
  useEffect(() => {
    if (!pendingRuleFocus || !rules) return;
    setFocusRuleId(pendingRuleFocus.ruleId);
    onConsumePendingRuleFocus?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingRuleFocus, rules]);

  if (loading && !rules) return <Loader />;
  if (error) return <Alert color="red">achievements indisponível.</Alert>;

  return (
    <GameEventRulesCanvas
      gameId={gameId}
      rules={visibleRules}
      catalog={catalog ?? []}
      archetypeOptions={archetypeOptions}
      modes={modes ?? []}
      reload={reloadAll}
      focusRuleId={focusRuleId}
      onFocusRule={setFocusRuleId}
      autoEditRuleId={pendingRuleFocus?.edit ? pendingRuleFocus.ruleId : null}
    />
  );
}
