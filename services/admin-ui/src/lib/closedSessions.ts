// Histórico de sessões fechadas ao vivo (spec 095): a aba Sessions do jogo busca
// as abertas a cada polling, mas o histórico (Postgres) só quando algo fecha.

// someClosed diz se alguma sessão que estava aberta sumiu das abertas: é o
// sinal de que ela fechou e o histórico precisa ser recarregado.
export function someClosed(prevOpenIds: ReadonlySet<string>, open: readonly { sessionId: string }[]): boolean {
  if (prevOpenIds.size === 0) return false;
  const now = new Set(open.map((s) => s.sessionId));
  for (const id of prevOpenIds) if (!now.has(id)) return true;
  return false;
}

// mergeClosed junta a primeira página do histórico com as carregadas por
// "carregar mais", sem repetir sessão: recarregar a primeira página depois de
// um fechamento empurra linhas para a fronteira com as páginas seguintes.
export function mergeClosed<T extends { sessionId: string }>(first: readonly T[], extra: readonly T[]): T[] {
  const seen = new Set<string>();
  const out: T[] = [];
  for (const s of [...first, ...extra]) {
    if (seen.has(s.sessionId)) continue;
    seen.add(s.sessionId);
    out.push(s);
  }
  return out;
}
