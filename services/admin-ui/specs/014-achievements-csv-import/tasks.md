# Tasks 014 — Importação de conquistas via CSV (admin-ui)

## Backend (admin-ui consome — depende da spec 011 estar implementada)

- [x] **1. Tipos de importação** — adicionar `ImportResult`, `ImportResponse` em `lib/api.ts`
- [x] **2. Método da API** — adicionar `api.importAchievements(gameId, achievements)` em `lib/api.ts`

## Utilitário CSV

- [x] **3. Criar `lib/csv.ts`** — exportar `parseCSV(text): ParseResult`, constante `CSV_TEMPLATE` e tipos `ParsedRow`, `ParseError`, `ParseResult`
- [x] **4. Validações no parser** — id (regex + tamanho), name, eventType, target (int >= 1), ids duplicados no arquivo, máx 500 linhas, cabeçalho obrigatório

## UI — GameAchievements.tsx

- [x] **5. Botão "Baixar modelo"** — `handleDownloadTemplate` com `Blob` + `createObjectURL` + `<a download>`
- [x] **6. Botão "Importar CSV" + input oculto** — `fileInputRef`, `<input type="file" accept=".csv" style={{display:'none'}}>`, `importing` state
- [x] **7. `handleFileChange`** — validações de extensão e tamanho (toast), chama `parseCSV`, bifurca para modal de erros ou chama API
- [x] **8. Modal de erros de validação** — `importErrors` state, `Modal` Mantine com lista de `Linha N: mensagem`
- [x] **9. Modal de resultado** — `importResult` state, `Modal` Mantine com badge de resumo e tabela de falhas
- [x] **10. Recarregar tabela após import** — chamar `reload()` se `result.created > 0`
