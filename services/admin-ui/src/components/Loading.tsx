export function Loading({ what }: { what?: string }) {
  return <div className="muted">Carregando{what ? ` ${what}` : ""}…</div>;
}
