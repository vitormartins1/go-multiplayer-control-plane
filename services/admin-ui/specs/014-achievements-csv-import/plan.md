# Plan 014 — Importação de conquistas via CSV (admin-ui)

## Decisões técnicas

### Parser CSV no frontend (sem biblioteca)

O CSV tem estrutura simples (sem aspas multi-linha, sem escape complexo).  
Usar `String.split('\n')` + `String.split(',')` com trim é suficiente e evita adicionar dependência.  
Tratar aspas duplas ao redor de valores (ex.: `"First, Win"`) com regex simples: `/^"(.*)"$/`.

### Sem preview antes de importar

Adicionar um passo de preview aumentaria a complexidade da UX sem benefício claro para o cenário de uso (admin que preparou o CSV antecipadamente). O modal de resultado pós-importação e o modal de erros de validação são suficientes.

### Download do modelo: geração client-side

`Blob` + `URL.createObjectURL` + `<a download>` — sem chamada ao servidor.

### Estado de loading com `useRef` para o input de arquivo

O `<input type="file">` fica oculto; o botão "Importar CSV" chama `.click()` nele via ref.  
O estado de loading é controlado por `useState` no componente `GameAchievements`.

### Reutilização do modal de resultado

Usar `Modal` do Mantine (já usado no formulário de conquista). Manter o estado do resultado em `useState<ImportResponse | null>`.

## Arquivos a modificar / criar

```
services/admin-ui/src/
  lib/api.ts                          ← adicionar importAchievements + tipo ImportResponse
  lib/csv.ts                          ← novo: parseCSV + validateRows (utilitário puro)
  pages/GameAchievements.tsx          ← botões, input oculto, modais de validação e resultado
```

Nenhuma nova dependência npm.

## Tipos novos (lib/api.ts)

```typescript
interface ImportResult {
  row: number
  id: string
  status: 'created' | 'duplicate' | 'invalid' | 'error'
  error?: string
}

interface ImportResponse {
  created: number
  failed: number
  results: ImportResult[]
}
```

Método:

```typescript
importAchievements(gameId: string, achievements: NewAchievement[]): Promise<ImportResponse>
// POST /api/games/{gameId}/achievements/import
// body: { achievements }
```

## Utilitário CSV (lib/csv.ts)

```typescript
interface ParsedRow {
  row: number           // 1-based (linha de dados, sem contar o cabeçalho)
  id: string
  name: string
  description: string
  eventType: string
  target: number
}

interface ParseError {
  row: number
  message: string
}

interface ParseResult {
  rows: ParsedRow[]
  errors: ParseError[]
}

function parseCSV(text: string): ParseResult
```

Passos internos:
1. Split por `\n`, filtrar linhas em branco
2. Primeira linha = cabeçalho → mapear índices de coluna (case-insensitive)
3. Validar que todas as colunas obrigatórias existem no cabeçalho
4. Para cada linha de dados: parsear colunas, validar cada campo, coletar erros
5. Validar IDs duplicados dentro do arquivo (passar segundo loop após parsear tudo)

Exportar também `CSV_TEMPLATE` como string constante (conteúdo do arquivo modelo).

## Mudanças em GameAchievements.tsx

### Estado adicional

```typescript
const fileInputRef = useRef<HTMLInputElement>(null)
const [importing, setImporting] = useState(false)
const [importResult, setImportResult] = useState<ImportResponse | null>(null)
const [importErrors, setImportErrors] = useState<ParseError[]>([])
```

### Botões na toolbar

```tsx
<Group justify="space-between">
  <Group gap="xs">
    <Button variant="default" size="sm" onClick={handleDownloadTemplate}>
      Baixar modelo
    </Button>
    <Button variant="default" size="sm" loading={importing} onClick={() => fileInputRef.current?.click()}>
      Importar CSV
    </Button>
    <input
      ref={fileInputRef}
      type="file"
      accept=".csv"
      style={{ display: 'none' }}
      onChange={handleFileChange}
    />
  </Group>
  <Button onClick={openCreateModal}>+ Nova conquista</Button>
</Group>
```

### handleDownloadTemplate

```typescript
function handleDownloadTemplate() {
  const blob = new Blob([CSV_TEMPLATE], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'achievements-template.csv'
  a.click()
  URL.revokeObjectURL(url)
}
```

### handleFileChange

```typescript
async function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
  const file = e.target.files?.[0]
  e.target.value = ''          // reset para permitir re-seleção do mesmo arquivo
  if (!file) return

  if (!file.name.endsWith('.csv')) {
    notifications.show({ message: 'Selecione um arquivo .csv', color: 'red' })
    return
  }
  if (file.size > 200_000) {
    notifications.show({ message: 'Arquivo muito grande (máx. 200 KB)', color: 'red' })
    return
  }

  const text = await file.text()
  const { rows, errors } = parseCSV(text)

  if (errors.length > 0) {
    setImportErrors(errors)   // abre modal de erros de validação
    return
  }

  setImporting(true)
  try {
    const result = await api.importAchievements(gameId, rows)
    setImportResult(result)
    if (result.created > 0) refetch()   // recarregar tabela
  } finally {
    setImporting(false)
  }
}
```

### Modal de erros de validação

`Modal` do Mantine, aberto quando `importErrors.length > 0`.  
Lista `<List>` com `Linha N: mensagem` por item.  
Fechar → `setImportErrors([])`.

### Modal de resultado

`Modal` do Mantine, aberto quando `importResult !== null`.  
Badge de resumo: "X criadas" (green) + "Y falhas" (red se > 0).  
Tabela de falhas (se houver): colunas Linha, ID, Motivo.  
Fechar → `setImportResult(null)`.
