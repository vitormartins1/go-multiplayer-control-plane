# Spec 014 — Importação de conquistas via CSV (admin-ui)

**Status:** Done

## Contexto

O administrador de um jogo precisa cadastrar muitas conquistas de uma vez. Em vez de usar o formulário individual repetidamente, ele pode preparar um CSV (manualmente, em planilha ou via IA) e importar em lote pela aba Achievements do jogo.

## Formato do CSV

```
id,name,description,eventType,target
first_win,First Win,Win your first match,match_won,1
ten_wins,Ten Wins,Win ten matches,match_won,10
level_up,Level Up,Reach level 10,level_reached,10
```

| Coluna | Obrigatório | Regras |
|---|---|---|
| `id` | sim | 2–40 chars, `[a-z0-9_-]`, sem hífen/underscore nas pontas (`/^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$/`) |
| `name` | sim | não vazio |
| `description` | não | pode ficar em branco |
| `eventType` | sim | não vazio |
| `target` | sim | inteiro >= 1 |

- A **primeira linha** deve ser o cabeçalho (case-insensitive, ordem indiferente).
- Linhas completamente em branco são ignoradas.
- Máximo de **500 linhas de dados** por arquivo.

## Modelo de CSV para download

O botão "Baixar modelo" gera e faz download de `achievements-template.csv` com o cabeçalho e **3 linhas de exemplo** comentadas:

```
id,name,description,eventType,target
first_win,First Win,Win your first match,match_won,1
ten_wins,Ten Wins,Win ten matches,match_won,10
reach_level_10,Level 10,Reach level 10 in your first session,level_reached,10
```

O arquivo é gerado no cliente (sem chamada ao servidor).

## UI — localização dos botões

Na barra de ações da aba Achievements do GameDetail (linha que hoje tem "Nova conquista"), adicionar dois botões à esquerda do botão principal:

```
[ Baixar modelo ]  [ Importar CSV ]             [ + Nova conquista ]
```

## Fluxo de importação

```
1. Operador clica "Importar CSV"
2. Abre seletor de arquivo nativo (accept=".csv")
3. Operador seleciona arquivo
4. Frontend lê e parseia o CSV
5. Frontend valida todas as linhas
   5a. Se houver erros de formato → exibe modal de erros de validação (NÃO envia)
6. Frontend chama POST /api/games/{gameId}/achievements/import
7. Exibe modal de resultado (criados + falhas por linha)
8. Operador fecha o modal; tabela é recarregada automaticamente
```

## Modal de erros de validação (passo 5a)

Exibido ANTES de enviar, quando há linhas com formato inválido no próprio CSV:

- Título: "Arquivo inválido"
- Lista de erros: `Linha N: <mensagem>` (ex.: `Linha 3: campo 'id' inválido — use letras minúsculas, números, _ e -`)
- Botão "Fechar"
- Nenhum dado é enviado ao servidor

## Modal de resultado (passo 7)

Exibido após a resposta do servidor:

- Título: "Importação concluída"
- Linha de resumo: "X conquistas criadas, Y falhas"
- Se `failed > 0`: tabela com linhas que falharam (`Linha`, `ID`, `Motivo`)
  - `duplicate` → "Já existe uma conquista com este ID neste jogo"
  - `invalid`   → "Dados inválidos (verifique os campos)"
  - `error`     → "Serviço indisponível — tente novamente"
- Botão "Fechar" (sempre); se `created > 0`, fecha e recarrega a tabela

## Validações no frontend (antes de enviar)

| Validação | Mensagem de erro |
|---|---|
| Arquivo sem extensão `.csv` | "Selecione um arquivo .csv" (toast, não abre modal) |
| Arquivo maior que 200 KB | "Arquivo muito grande (máx. 200 KB)" |
| Cabeçalho ausente ou colunas faltando | "Cabeçalho inválido — use: id,name,description,eventType,target" |
| Mais de 500 linhas de dados | "O arquivo tem mais de 500 conquistas. Divida em arquivos menores." |
| `id` vazio ou formato inválido | `Linha N: 'id' inválido — use letras minúsculas, números, _ e -` |
| `name` vazio | `Linha N: 'name' é obrigatório` |
| `eventType` vazio | `Linha N: 'eventType' é obrigatório` |
| `target` não é inteiro >= 1 | `Linha N: 'target' deve ser um número inteiro maior que zero` |
| IDs duplicados dentro do mesmo CSV | `Linha N: id '...' aparece mais de uma vez no arquivo` |

Se **qualquer** linha tiver erro de validação, o modal de erros é aberto e o upload é cancelado.

## Estados do botão "Importar CSV"

- Normal: habilitado
- Após selecionar arquivo: carregando (spinner, desabilitado) enquanto parseia + envia
- Após resultado: normal novamente

## Não está no escopo desta spec

- Preview das linhas antes de importar
- Drag-and-drop de arquivo
- Edição inline do CSV dentro do painel
- Download do resultado de erros como CSV
