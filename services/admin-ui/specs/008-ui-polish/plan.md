# Plan: 008-ui-polish

## Abordagem

Todas as mudanças são **front-end only** no `admin-ui`. Nenhuma alteração no `admin-bff` ou em outros serviços.

## Arquivos alterados

| Arquivo | Mudança |
|---------|---------|
| `src/App.tsx` | Nav reordenada; rota `/infrastructure`; redirect `/` → `/games` |
| `src/pages/Overview.tsx` | Título "Infrastructure" |
| `src/pages/GameDetail.tsx` | 3 `useAsync` para contagens; `CountBadge` helper; `rightSection` nas abas |
| `src/components/AchievementIcon.tsx` | **Novo** — DiceBear glyphs via API pública |
| `src/pages/GameAchievements.tsx` | Importa `AchievementIcon`; nova coluna na tabela |
| `src/pages/GameSessions.tsx` | Estado `expandedId`; `SessionRow` expansível com painel de detalhes |

## Decisões técnicas

### Glyphs via API (não via npm)
O estilo `glyphs` do DiceBear não está disponível no pacote `@dicebear/collection@9.4.2` (nem como pacote separado no registry npm). A solução é consumir a API hospedada do DiceBear:
```
https://api.dicebear.com/9.x/glyphs/svg?seed={achievementId}
```
As requests são feitas pelo navegador (não pelo container), então funcionam em dev sem configuração extra. Consistente com a abordagem de não instalar pacotes desnecessários.

### Contagens no GameDetail
Três chamadas adicionais ao montar `GameDetail`, reutilizando endpoints já existentes:
- `api.gameCatalog(gameId)` → `length`
- `api.gamePlayers(gameId)` → `length`  
- `api.gameSessions(gameId)` → `active.length` + Set de player IDs únicos

Duplicação proposital com as buscas das sub-abas (que fazem seus próprios fetches ao montar). O custo é baixo (3 GETs extras) e evita prop drilling ou estado compartilhado.

### Sessions expansíveis
Estado `expandedId: string | null` no componente pai `GameSessions`. Toggle alternado (clicar na mesma linha recolhe). Painel usa `Collapse` do Mantine para animação suave.
