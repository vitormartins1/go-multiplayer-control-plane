import { describe, expect, it } from "vitest";
import { CSV_TEMPLATE, parseCSV } from "./csv";

describe("parseCSV", () => {
  it("aceita o próprio modelo baixável sem erros (evento e métrica)", () => {
    const { rows, errors } = parseCSV(CSV_TEMPLATE);
    expect(errors).toEqual([]);
    expect(rows).toHaveLength(6);
    expect(rows[0]).toMatchObject({ id: "primeira_vitoria", eventType: "match_won", target: 1 });
    expect(rows[3]).toMatchObject({ id: "exterminador", eventType: "", statKey: "kills", statMode: "cumulative" });
  });

  it("respeita aspas duplas com vírgula e aspas escapadas", () => {
    const { rows, errors } = parseCSV('id,name,description,eventType,target\nab,"Nome, com vírgula","diz ""oi""",x,2\n');
    expect(errors).toEqual([]);
    expect(rows[0]).toMatchObject({ name: "Nome, com vírgula", description: 'diz "oi"', target: 2 });
  });

  it("recusa cabeçalho sem as colunas obrigatórias", () => {
    const { rows, errors } = parseCSV("id,name\nab,x\n");
    expect(rows).toEqual([]);
    expect(errors[0].message).toContain("Faltando: description, eventtype, target");
  });

  it("recusa critério duplo, critério ausente, statMode inválido, target inválido e id repetido", () => {
    const csv = [
      "id,name,description,eventType,target,statKey,statMode",
      "dois,Dois,,match_won,1,kills,cumulative",
      "nenhum,Nenhum,,,1,,",
      "modo,Modo,,,1,kills,sempre",
      "alvo,Alvo,,match_won,0,,",
      "ok,Ok,,match_won,1,,",
      "ok,Ok de novo,,match_won,1,,",
    ].join("\n");
    const { rows, errors } = parseCSV(csv);
    expect(rows.map((r) => r.id)).toEqual(["ok"]);
    expect(errors.map((e) => e.row)).toEqual([1, 2, 3, 4, 6]);
    expect(errors[4].message).toContain("primeira ocorrência na linha 5");
  });

  it("arquivo vazio é erro na linha 0", () => {
    expect(parseCSV("\n \n").errors).toEqual([{ row: 0, message: "Arquivo vazio." }]);
  });
});
