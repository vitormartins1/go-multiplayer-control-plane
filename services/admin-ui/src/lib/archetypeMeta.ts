import type { Icon } from "@tabler/icons-react";
import { IconParachute, IconPlanet, IconSwords, IconTargetArrow } from "@tabler/icons-react";
import type { MultiplayerProfile } from "./api";

// Rótulo e ícone de cada arquétipo (ADR-0023) — compartilhado entre a lista
// de modos (Configurações), o header do GameDetail (030) e o Event Rules.
// Ícone em vez de cor (revisão de layout, 2026-07-10): as 4 cores que cada
// arquétipo tinha colidiam uma a uma com uma categoria do canvas de fluxo
// (flow.css) — mesma cor, conceitos diferentes, visíveis na mesma tela (Event
// Rules mostra os dois juntos). Ícone não compete pela paleta e escala sem
// precisar caçar mais uma cor livre a cada arquétipo novo.
export const ARCHETYPE_META: Record<
  MultiplayerProfile["archetype"],
  { label: string; icon: Icon; experimental?: boolean }
> = {
  queue_arena: { label: "Queue Arena", icon: IconSwords },
  host_world_dropin: { label: "Host World Drop-in", icon: IconPlanet },
  // experimental (decisão D5 da revisão 2026-09): contrato e código prontos,
  // nenhum jogo real usa — congelado até existir um caso de verdade.
  battle_royale: { label: "Battle Royale", icon: IconParachute, experimental: true },
  // mesmo símbolo do banner de missão (CoopPanel, SessionMatchDetail.tsx) — mesmo conceito.
  coop_mission: { label: "Co-op Mission", icon: IconTargetArrow, experimental: true },
};

// archetypeMetaOf busca o meta por uma string arbitrária (spec 022: o
// arquétipo de uma sessão vem do backend como string livre — passthrough,
// pode faltar em sessão legada ou vir de um arquétipo desconhecido). undefined
// em vez de indexar direto no ARCHETYPE_META, que só aceita o union tipado.
export function archetypeMetaOf(archetype: string | undefined): { label: string; icon: Icon } | undefined {
  if (!archetype) return undefined;
  return (ARCHETYPE_META as Record<string, { label: string; icon: Icon }>)[archetype];
}
