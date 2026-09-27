# Mídia — vídeo e imagens do produto

| Arquivo | O que mostra | Onde aparece |
|---|---|---|
| [`go-multiplayer-control-plane.mp4`](go-multiplayer-control-plane.mp4) | o filme completo (87 s, 1080p, ~7 MB): o problema, a arquitetura, uma invasão P2P, um tour pelo console e a stack | links no README, PRODUCT-OVERVIEW e contrato do `host_world_dropin` |
| `hero.jpg` | quadro da arquitetura com o fan-out aceso e o selo "Assista ao vídeo" | capa do README |
| `invasao-p2p.jpg` | quadro da invasão P2P com a conquista desbloqueada | README, PRODUCT-OVERVIEW, contrato do `host_world_dropin` |

As telas do console usadas nos docs (`docs/images/rooms-p2p.jpg`, `docs/images/achievements-steam.jpg`) são as
mesmas capturas reais que o filme usa, reduzidas para 1600 px.

**Por que não GIF:** foi tentado. A paleta de 256 cores do GIF estraga o degradê escuro do filme e cada recorte
passava de 10 MB. Imagem parada + link para o MP4 fica nítido, leve e mostra o movimento onde ele importa. O GitHub
não toca MP4 do repositório dentro do README; quem clica abre o player do próprio GitHub.

## Como é feito

O filme é **uma página HTML** ([`src/film.html`](src/film.html), 1920×1080) em que cada quadro é uma função pura do
tempo: `window.__seek(t)` posiciona toda a animação no instante `t`, sem relógio nem `requestAnimationFrame`. Isso
torna a renderização determinística: [`src/render.mjs`](src/render.mjs) abre a página no Chromium do Playwright,
chama `__seek(i/30)` para cada quadro, tira o screenshot e manda o PNG para o `ffmpeg` (H.264, 30 fps, CRF 25). As
imagens paradas saem da mesma página, no instante escolhido, com o selo desenhado em HTML na fonte do filme.

As telas do console (`src/shots/`) são **capturas reais** do admin-ui rodando com dados da demo
(`scripts/demo-host-world.sh`, nas duas topologias), tiradas em DPR 2. O `shots/boxes.json` guarda as caixas dos
elementos de cada tela; a câmera, os destaques e as legendas do filme usam essas caixas.

## Re-renderizar

Requisitos: as dependências do admin-ui instaladas (`npm ci` em `services/admin-ui`, que traz o Playwright) e um
`ffmpeg` no `PATH` (ou em `FFMPEG`).

```bash
node docs/media/src/render.mjs                       # MP4 + imagens (~20 min)
node docs/media/src/render.mjs --stills              # só as imagens paradas (segundos)
node docs/media/src/render.mjs --frames 3,20,35,60   # só quadros soltos em docs/media/frames/ (ignorado no git)
```

Para mudar o roteiro, edite o objeto `T` (início/fim de cada cena) e as cenas em `film.html`; para mudar os
quadros das imagens paradas, a lista `STILLS` em `render.mjs`. Telas novas: capture o console em 1920×1080 com DPR 2, salve em
`src/shots/` e acrescente as caixas no `boxes.json`.

Fontes: Space Grotesk, IBM Plex Sans e IBM Plex Mono (SIL Open Font License), em `src/fonts/`.
