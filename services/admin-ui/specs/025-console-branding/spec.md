# Spec: 025-console-branding

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-07)
**Data:** 2026-07-07
**Parte de:** lote console-polish.

## Contexto

O header exibe `🎮 Admin Console` (App.tsx): emoji quebra o alinhamento vertical do texto e
o nome não reflete o produto. O repo é `go-multiplayer-control-plane` — a marca do console
deve ecoar isso.

## Comportamento

- THE SYSTEM SHALL exibir no header: um `ThemeIcon` (variant gradient) com o ícone Tabler
  `IconTopologyStar3` + o título **"Multiplayer Control Plane"** + o sufixo **"Console"**
  esmaecido (dimmed) — sem emoji.
- THE SYSTEM SHALL exibir à direita do header um badge de ambiente **"local"** (variant
  light). Valor fixo por ora; vira variável de ambiente quando existir outro ambiente.
- A altura do header (56) e o layout responsivo permanecem inalterados.

## Contratos de Interface (cliente)

- `src/App.tsx`: substituição do `<Title>🎮 Admin Console</Title>` por Group com ThemeIcon +
  Title + Text dimmed + Badge (justify space-between).

## Casos de Teste / Verificação

- [x] header sem emoji; ícone, título, sufixo e badge alinhados na linha de 56px
- [x] navegação/navbar intactas
- [x] `npm run build` limpo

## Fora do escopo

- Favicon/`<title>` do index.html (aditivo trivial — incluir se não gerar discussão);
  temas/cores globais; logo próprio.
