// Histórico de sessões fechadas ao vivo (spec 095): a aba Sessions do jogo busca
// as abertas a cada polling, mas o histórico (Postgres) só quando algo fecha.

// goneIds devolve as sessões que estavam abertas e sumiram das abertas: elas
// fecharam, e o histórico precisa ser recarregado para mostrá-las.
export function goneIds(prevOpenIds: ReadonlySet<string>, open: readonly { sessionId: string }[]): string[] {
  const now = new Set(open.map((s) => s.sessionId));
  return [...prevOpenIds].filter((id) => !now.has(id));
}

// MAX_HISTORY_TRIES limita quantos ciclos de polling uma sessão fechada espera
// aparecer no histórico. O session tira a sessão das abertas antes de gravá-la
// no Postgres; um recarregamento nessa janela ainda não a encontra.
export const MAX_HISTORY_TRIES = 3;

// awaitHistory avança a espera das sessões fechadas que ainda não estão no
// histórico: as que apareceram (ou esgotaram as tentativas) saem, as outras
// gastam uma tentativa. reload diz se o histórico deve ser recarregado agora.
export function awaitHistory(
  awaiting: ReadonlyMap<string, number>,
  gone: readonly string[],
  inHistory: ReadonlySet<string>,
): { awaiting: Map<string, number>; reload: boolean } {
  const next = new Map(awaiting);
  for (const id of gone) next.set(id, 0);
  let reload = false;
  for (const [id, tries] of next) {
    if (inHistory.has(id) || tries >= MAX_HISTORY_TRIES) {
      next.delete(id);
      continue;
    }
    next.set(id, tries + 1);
    reload = true;
  }
  return { awaiting: next, reload };
}

// mergeClosed junta páginas do histórico sem repetir sessão, na ordem dada.
// Ao recarregar a primeira página depois de um fechamento, a página antiga
// vai junto (antes das de "carregar mais"): a linha que a nova primeira página
// empurrou para fora continua na lista, e o cursor de "carregar mais" segue
// valendo.
export function mergeClosed<T extends { sessionId: string }>(...pages: readonly (readonly T[])[]): T[] {
  const seen = new Set<string>();
  const out: T[] = [];
  for (const s of pages.flat()) {
    if (seen.has(s.sessionId)) continue;
    seen.add(s.sessionId);
    out.push(s);
  }
  return out;
}
