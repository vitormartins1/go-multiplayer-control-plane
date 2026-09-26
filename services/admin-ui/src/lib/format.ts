import { closeReasonMeta } from "./vocab";

// Helpers de formatação compartilhados pelas telas de sessões/salas.

// shortId mostra só o prefixo de um UUID (8 chars bastam para distinguir
// sessões na tela; o id completo fica no tooltip/detalhe).
export function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

// truncate encurta ids longos (match/server) para caber na tabela.
export function truncate(id: string): string {
  return id.length > 12 ? `${id.slice(0, 12)}…` : id;
}

// closeReasonLabel traduz o motivo de fechamento da sessão para exibição —
// fonte única em lib/vocab.ts (cobre os 6 motivos do session-service).
export function closeReasonLabel(reason?: string): string {
  return closeReasonMeta(reason).label;
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
