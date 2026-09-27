# Mídia — vídeo e GIFs do produto

| Arquivo | O que mostra | Onde aparece |
|---|---|---|
| [`go-multiplayer-control-plane.mp4`](go-multiplayer-control-plane.mp4) | o filme completo (~90 s, 1080p): o problema, a arquitetura, uma invasão P2P, um tour pelo console e a stack | README (link) |
| `arquitetura.gif` | pacotes percorrendo a arquitetura: login, alocação, fan-out | README |
| `invasao-p2p.gif` | invasão num mundo P2P: matchmaking → SteamID do host → Steam → evento → conquista | README, PRODUCT-OVERVIEW, contrato do `host_world_dropin` |
| `console-rooms.gif` | salas ao vivo, hospedagem (dedicado/P2P), heartbeat e o detalhe do mundo | README, PRODUCT-OVERVIEW, admin-ui |
| `console-conquistas-analytics.gif` | catálogo com selo Steam e o analytics do mundo | PRODUCT-OVERVIEW, achievements |

## Como é feito

O filme é **uma página HTML** ([`src/film.html`](src/film.html), 1920×1080) em que cada quadro é uma função pura do
tempo: `window.__seek(t)` posiciona toda a animação no instante `t`, sem relógio nem `requestAnimationFrame`. Isso
torna a renderização determinística: [`src/render.mjs`](src/render.mjs) abre a página no Chromium do Playwright,
chama `__seek(i/30)` para cada quadro, tira o screenshot e manda o PNG para o `ffmpeg` (H.264, 30 fps). Os GIFs são
recortes do MP4, a 960 px e 15 fps, com paleta própria por GIF.

As telas do console (`src/shots/`) são **capturas reais** do admin-ui rodando com dados da demo
(`scripts/demo-host-world.sh`, nas duas topologias), tiradas em DPR 2. O `shots/boxes.json` guarda as caixas dos
elementos de cada tela; a câmera, os destaques e as legendas do filme usam essas caixas.

## Re-renderizar

Requisitos: as dependências do admin-ui instaladas (`npm ci` em `services/admin-ui`, que traz o Playwright) e um
`ffmpeg` no `PATH` (ou em `FFMPEG`).

```bash
node docs/media/src/render.mjs                       # MP4 + GIFs (~20 min)
node docs/media/src/render.mjs --frames 3,20,35,60   # só quadros soltos em docs/media/frames/ (ignorado no git)
```

Para mudar o roteiro, edite o objeto `T` (início/fim de cada cena) e as cenas em `film.html`; para mudar os
recortes dos GIFs, a lista `GIFS` em `render.mjs`. Telas novas: capture o console em 1920×1080 com DPR 2, salve em
`src/shots/` e acrescente as caixas no `boxes.json`.

Fontes: Space Grotesk, IBM Plex Sans e IBM Plex Mono (SIL Open Font License), em `src/fonts/`.
