// Utilitário de parsing e validação de CSV para importação de achievements.
// Sem dependências externas — o formato é simples (sem multi-line, aspas duplas
// são suportadas para valores com vírgulas).

const ID_RE = /^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$/;
const STAT_RE = /^[a-z][a-z0-9_]*$/;
const REQUIRED_HEADERS = ["id", "name", "description", "eventtype", "target"] as const;

// Modelo baixável (033): CSV não tem comentários padronizados, então a
// documentação são as próprias linhas de exemplo — uma por tipo de critério,
// com descrições que explicam o que cada linha demonstra. statKey/statMode são
// opcionais: preenchidos → critério por métrica (eventType fica vazio);
// vazios → critério por evento (formato antigo, retrocompatível).
export const CSV_TEMPLATE =
  "id,name,description,eventType,target,statKey,statMode\n" +
  'primeira_vitoria,Primeira Vitória,"Exemplo: evento simples — dispara na 1ª ocorrência de match_won",match_won,1,,\n' +
  'veterano,Veterano,"Exemplo: evento acumulado — 10 ocorrências de match_played",match_played,10,,\n' +
  'campeao_royale,Campeão do Royale,"Exemplo: evento de outro arquétipo (battle_royale)",battle_royale_win,1,,\n' +
  'exterminador,Exterminador,"Exemplo: métrica acumulada — soma de kills entre partidas até 500",,500,kills,cumulative\n' +
  'furia,Fúria,"Exemplo: métrica por partida — 8 kills numa única partida",,8,kills,per_match\n' +
  'pontuacao_lendaria,Pontuação Lendária,"Exemplo: métrica score (intrínseca a todo resultado) numa partida",,100,score,per_match\n';

export interface ParsedRow {
  row: number;
  id: string;
  name: string;
  description: string;
  eventType: string;
  target: number;
  statKey?: string;
  statMode?: "cumulative" | "per_match";
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
  // colunas opcionais (033): ausentes no arquivo → -1 → campo sempre vazio.
  const iStatKey = idx("statkey");
  const iStatMode = idx("statmode");

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
    const statKey = (iStatKey >= 0 ? cols[iStatKey] : "") ?? "";
    const statMode = ((iStatMode >= 0 ? cols[iStatMode] : "") ?? "").toLowerCase();

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
    // Critério (033): exatamente um de eventType/statKey por linha.
    if (statKey && eventType) {
      errors.push({
        row,
        message: `Linha ${row}: preencha 'eventType' OU 'statKey', nunca os dois (o critério é por evento ou por métrica).`,
      });
      continue;
    }
    if (!eventType && !statKey) {
      errors.push({
        row,
        message: `Linha ${row}: informe um critério — 'eventType' (por evento) ou 'statKey' (por métrica).`,
      });
      continue;
    }
    if (statKey && !STAT_RE.test(statKey)) {
      errors.push({
        row,
        message: `Linha ${row}: 'statKey' inválido — minúsculo, começa com letra, só letras/números/_ (ex.: kills).`,
      });
      continue;
    }
    if (statKey && statMode !== "cumulative" && statMode !== "per_match") {
      errors.push({
        row,
        message: `Linha ${row}: 'statMode' deve ser 'cumulative' ou 'per_match' quando 'statKey' está preenchido (recebido: "${statMode || "vazio"}").`,
      });
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

    rows.push({
      row,
      id,
      name,
      description,
      eventType,
      target,
      ...(statKey ? { statKey, statMode: statMode as "cumulative" | "per_match" } : {}),
    });
  }

  return { rows, errors };
}
