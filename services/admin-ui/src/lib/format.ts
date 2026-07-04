// Helpers de formatação compartilhados pelas telas de sessões/salas.

// truncate encurta ids longos (match/server) para caber na tabela.
export function truncate(id: string): string {
  return id.length > 12 ? `${id.slice(0, 12)}…` : id;
}

// closeReasonLabel traduz o motivo de fechamento da sessão para exibição.
export function closeReasonLabel(reason?: string): string {
  switch (reason) {
    case "ttl":
      return "TTL expirado (sessão abandonada)";
    case "admin":
      return "Encerrada pelo operador";
    case "result":
      return "Resultado reportado";
    default:
      return reason ?? "—";
  }
}

// formatDuration formata uma duração (ms) como "Ns" / "Nmin Ns" / "Nh Nmin".
export function formatDuration(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000));
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  if (min >= 60) return `${Math.floor(min / 60)}h ${min % 60}min`;
  if (min > 0) return `${min}min ${sec}s`;
  return `${sec}s`;
}
