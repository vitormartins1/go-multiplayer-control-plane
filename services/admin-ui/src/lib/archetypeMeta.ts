import type { MultiplayerProfile } from "./api";

// Rótulo e cor de cada arquétipo (ADR-0023) — compartilhado entre a lista de
// modos (Configurações) e o header do GameDetail (030).
export const ARCHETYPE_META: Record<
  MultiplayerProfile["archetype"],
  { label: string; color: string }
> = {
  queue_arena: { label: "Queue Arena", color: "blue" },
  host_world_dropin: { label: "Host World Drop-in", color: "grape" },
  battle_royale: { label: "Battle Royale", color: "orange" },
  coop_mission: { label: "Co-op Mission", color: "teal" },
};
