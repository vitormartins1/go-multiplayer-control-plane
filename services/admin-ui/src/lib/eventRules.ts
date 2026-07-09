import type { EventRule, MultiplayerMode, MultiplayerProfile } from "./api";

// gameArchetypes = arquétipos que o jogo REALMENTE tem cadastrado (união dos
// modos/perfis multiplayer). Usado pra filtrar as regras globais (migration
// 008 do achievements-service) e restringir o form de criação — evita
// confusão no admin ao cadastrar achievements/regras citando um arquétipo
// que o jogo não usa (regras globais valem pra TODO jogo do arquétipo, então
// sem esse filtro, um jogo só host_world_dropin ainda veria vocabulário de
// queue_arena/battle_royale/coop_mission).
export function gameArchetypes(modes: MultiplayerMode[]): Set<MultiplayerProfile["archetype"]> {
  return new Set(modes.map((m) => m.archetype));
}

// filterRulesForGame mantém: regras globais SÓ se o arquétipo delas está
// cadastrado no jogo; regras do jogo (scope="game") sempre, já que foram
// criadas explicitamente pra este jogo (um admin que cadastrar uma pelo
// form já vai ver o Select restrito às archetypes do jogo — ver
// GameEventRules.tsx — então essa segunda categoria não deveria acumular
// lixo, mas não escondemos overrides existentes por precaução/auditoria).
export function filterRulesForGame(
  rules: EventRule[],
  archetypes: Set<MultiplayerProfile["archetype"]>,
): EventRule[] {
  // Sem nenhum modo cadastrado ainda: não filtra (evita esconder tudo antes
  // do jogo ter qualquer perfil multiplayer — melhor mostrar de mais do que
  // parecer quebrado num jogo recém-criado).
  if (archetypes.size === 0) return rules;
  return rules.filter((r) => r.scope === "game" || archetypes.has(r.archetype));
}
