# Mídia — vídeo e imagens do produto

| Arquivo | O que mostra | Onde aparece |
|---|---|---|
| [`go-multiplayer-control-plane.mp4`](go-multiplayer-control-plane.mp4) | o filme do produto (~3 min, 1080p): o problema, como funciona, qualquer tipo de jogo, dedicado ou P2P, conquistas e regras, um tour pelo console e a engenharia. Os exemplos usam o Folclore Souls, mas o foco é o produto | links no README, PRODUCT-OVERVIEW e contrato do `host_world_dropin` |
| `hero.jpg` | quadro da arquitetura com o fan-out aceso e o selo "Assista ao vídeo" | capa do README |
| `invasao-p2p.jpg` | quadro da invasão P2P com a conquista desbloqueada | PRODUCT-OVERVIEW, contrato do `host_world_dropin` |
| `thumb-0N-*.jpg` | as capas clicáveis dos três vídeos de feature: um quadro do vídeo com o título, a duração e o play | seção "Veja em ação" do README |

### Vídeos de feature (gravados do console real)

| Arquivo | O que mostra |
|---|---|
| [`video-01-cadastro.mp4`](video-01-cadastro.mp4) | **Do zero ao jogo online**: cada parte do registro explicada (identidade, modo, arquétipo, topologia, papéis, matchmaking, métricas), a frota que o P2P dispensa e o primeiro mundo abrindo e sendo invadido |
| [`video-02-conquistas.mp4`](video-02-conquistas.mp4) | **Conquistas sem deploy**: critério por evento (um evento, duas metas) e por métrica (acumulada ou por partida), o grafo de uma regra lido coluna por coluna em tela cheia (fatos, regra, evento, conquistas), regras globais do arquétipo e supressão por jogo, uma regra nova com comparador, a conquista criada a partir do evento órfão e o desbloqueio ao vivo (14 de 50 numa, desbloqueada na outra) |
| [`video-03-jogador.mp4`](video-03-jogador.mp4) | **Visão 360 do jogador**: quem está online no jogo, conquistas com progresso e agrupamento, histórico com o desfecho de cada mundo (inclusive a queda do host), dados, save e a sessão ao vivo |

Os vídeos de feature são didáticos de propósito: cada momento importante ganha um destaque (o resto da tela escurece),
uma legenda que fica na tela o tempo de ler, e uma pausa. O 1 cria o jogo "Mata Adentro"; o 2 e o 3 usam o Folclore
Souls da demo, que declara stats nos modos e tem catálogo, ranking e histórico para explorar.

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

## Vídeos de feature: como são gravados

Aqui o console **roda de verdade**, sobre a stack da demo, e a gravação é em tempo real
([`src/record/lib.mjs`](src/record/lib.mjs)):

- o Playwright dirige o console (1280×720 com DPR 1,5, ou seja, quadros de 1920×1080 com a interface legível);
- o **screencast do CDP** manda cada quadro em JPEG com o instante em que foi desenhado, e o `ffmpeg` monta o
  vídeo respeitando esses instantes (qualidade melhor que o gravador embutido do Playwright);
- por cima do console, um **cursor**, o anel de clique, **legendas** e um destaque são injetados na página (o
  Chromium headless não desenha cursor);
- as **cartelas** de abertura e fechamento ([`src/record/card.html`](src/record/card.html)) seguem o estilo do
  filme e são renderizadas quadro a quadro, como ele;
- os **atores** ([`src/record/stage.sh`](src/record/stage.sh)) são jogadores de mentira que abrem mundo, invadem,
  reportam `HOST_KILLED` e gravam inventário e save enquanto a câmera roda, pelas mesmas APIs que o jogo usa.

Para regravar (precisa da stack no ar, do console em `localhost:5173` e de `grpcurl` e `jq` no `PATH`):

```bash
GAME_NAME="Mata Adentro" node docs/media/src/record/01-cadastro.mjs    # cria o jogo; o slug não pode existir
node docs/media/src/record/02-conquistas.mjs                           # usa o Folclore Souls da demo
node docs/media/src/record/03-jogador.mjs                              # usa o jogador "saci" do Folclore Souls
```

O slug de um jogo apagado continua reservado (soft delete, ADR-0025): para regravar o vídeo 1, use outro nome ou
uma base limpa.

As capas do README (`thumb-0N-*.jpg`) saem dos próprios vídeos: depois de regravar um deles, rode

```bash
node docs/media/src/record/thumbs.mjs    # o quadro de cada capa e o título ficam na lista THUMBS
```

### Até 10 MB por vídeo

O GitHub só toca vídeo embutido a partir de um anexo (`github.com/user-attachments/...`), e anexo tem teto de 10 MB.
Todo MP4 daqui fica abaixo disso: depois de renderizar ou regravar, rode

```bash
node docs/media/src/shrink.mjs docs/media/<arquivo>.mp4    # duas passadas até caber em 9,5 MB; se já cabe, não mexe
```

Como os vídeos são quase só tela parada e texto, a taxa baixa não aparece: o x264 gasta os bits onde a imagem muda.

Fontes: Space Grotesk, IBM Plex Sans e IBM Plex Mono (SIL Open Font License), em `src/fonts/`.
