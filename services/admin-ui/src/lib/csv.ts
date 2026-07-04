// Utilitário de parsing e validação de CSV para importação de achievements.
// Sem dependências externas — o formato é simples (sem multi-line, aspas duplas
// são suportadas para valores com vírgulas).

const ID_RE = /^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$/;
const REQUIRED_HEADERS = ["id", "name", "description", "eventtype", "target"] as const;

export const CSV_TEMPLATE =
  "id,name,description,eventType,target\n" +
  "first_win,First Win,Win your first match,match_won,1\n" +
  "ten_wins,Ten Wins,Win ten matches,match_won,10\n" +
  "reach_level_10,Level 10,Reach level 10 in your first session,level_reached,10\n";

export interface ParsedRow {
  row: number;
  id: string;
  name: string;
  description: string;
  eventType: string;
  target: number;
}

export interface ParseError {
  row: number;
  message: string;
}

export interface ParseResult {
  rows: ParsedRow[];
  errors: ParseError[];
}

// splitRow divide uma linha CSV respeitando aspas duplas.
function splitRow(line: string): string[] {
  const fields: string[] = [];
  let cur = "";
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === '"') {
      if (inQuotes && line[i + 1] === '"') {
        cur += '"';
        i++;
      } else {
        inQuotes = !inQuotes;
      }
    } else if (ch === "," && !inQuotes) {
      fields.push(cur.trim());
      cur = "";
    } else {
      cur += ch;
    }
  }
  fields.push(cur.trim());
  return fields;
}

export function parseCSV(text: string): ParseResult {
  const lines = text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l.length > 0);

  if (lines.length === 0) {
    return { rows: [], errors: [{ row: 0, message: "Arquivo vazio." }] };
  }

  // Mapear índices de coluna a partir do cabeçalho (case-insensitive).
  const headerCols = splitRow(lines[0]).map((h) => h.toLowerCase().replace(/\s/g, ""));
  const missing = REQUIRED_HEADERS.filter((h) => !headerCols.includes(h));
  if (missing.length > 0) {
    return {
      rows: [],
      errors: [
        {
          row: 0,
          message: `Cabeçalho inválido. Colunas esperadas: id,name,description,eventType,target. Faltando: ${missing.join(", ")}.`,
        },
      ],
    };
  }

  const idx = (col: string) => headerCols.indexOf(col);
  const iId = idx("id");
  const iName = idx("name");
  const iDesc = idx("description");
  const iEvent = idx("eventtype");
  const iTarget = idx("target");

  const rows: ParsedRow[] = [];
  const errors: ParseError[] = [];
  const seenIds = new Map<string, number>();

  for (let i = 1; i < lines.length; i++) {
    const row = i; // 1-based (linha de dados)
    const cols = splitRow(lines[i]);

    const id = cols[iId] ?? "";
    const name = cols[iName] ?? "";
    const description = cols[iDesc] ?? "";
    const eventType = cols[iEvent] ?? "";
    const rawTarget = cols[iTarget] ?? "";

    if (!id) {
      errors.push({ row, message: `Linha ${row}: campo 'id' é obrigatório.` });
      continue;
    }
    if (id.length < 2 || id.length > 40 || !ID_RE.test(id)) {
      errors.push({
        row,
        message: `Linha ${row}: 'id' inválido — use letras minúsculas, números, _ e - (2–40 caracteres, sem início/fim com _ ou -).`,
      });
      continue;
    }
    if (!name) {
      errors.push({ row, message: `Linha ${row}: campo 'name' é obrigatório.` });
      continue;
    }
    if (!eventType) {
      errors.push({ row, message: `Linha ${row}: campo 'eventType' é obrigatório.` });
      continue;
    }
    const target = parseInt(rawTarget, 10);
    if (isNaN(target) || target < 1) {
      errors.push({
        row,
        message: `Linha ${row}: 'target' deve ser um número inteiro maior que zero (recebido: "${rawTarget}").`,
      });
      continue;
    }

    if (seenIds.has(id)) {
      errors.push({
        row,
        message: `Linha ${row}: id '${id}' aparece mais de uma vez no arquivo (primeira ocorrência na linha ${seenIds.get(id)}).`,
      });
      continue;
    }
    seenIds.set(id, row);

    rows.push({ row, id, name, description, eventType, target });
  }

  return { rows, errors };
}
