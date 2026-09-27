// thumbs.mjs: gera as capas clicáveis dos vídeos de feature para o README
// (docs/media/thumb-0N-*.jpg). Cada capa é um quadro do próprio vídeo com o título,
// a duração e um botão de play por cima, na fonte e nas cores do filme.
//
//   node docs/media/src/record/thumbs.mjs
//
// Requisitos: os vídeos já gravados, o Playwright do admin-ui e um ffmpeg (FFMPEG).
import { execFile } from "node:child_process";
import { mkdir, readFile } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";
import { FFMPEG, mediaDir, repo } from "./lib.mjs";

const here = path.dirname(new URL(import.meta.url).pathname);
const { chromium } = await import(path.join(repo, "services/admin-ui/node_modules/playwright/index.mjs"));
const run = promisify(execFile);

// [vídeo, instante do quadro de fundo (s), rótulo, título]. A descrição fica no
// texto do README: numa grade de três capas, uma frase dentro da imagem não se lê.
const THUMBS = [
  ["01-cadastro", 40, "Vídeo 1 · Cadastro", "Do zero ao<br/>jogo online"],
  ["02-conquistas", 232, "Vídeo 2 · Conquistas", "Conquistas<br/>sem deploy"],
  ["03-jogador", 26, "Vídeo 3 · Jogador", "Visão 360<br/>do jogador"],
];

const duration = async (file) => {
  // o ffmpeg sem saída imprime "Duration: HH:MM:SS.xx" no stderr e sai com erro
  const r = await run(FFMPEG, ["-hide_banner", "-i", file]).catch((e) => e);
  const m = /Duration: (\d+):(\d+):(\d+)/.exec(r.stderr || "");
  if (!m) throw new Error("duração não encontrada: " + file);
  const s = Number(m[1]) * 3600 + Number(m[2]) * 60 + Number(m[3]);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
};

const font = async (f) => "data:font/woff2;base64," + (await readFile(path.join(here, "../fonts", f))).toString("base64");
const fonts = `
@font-face { font-family: G; font-weight: 700; src: url(${await font("space-grotesk-latin-700-normal.woff2")}); }
@font-face { font-family: M; font-weight: 500; src: url(${await font("ibm-plex-mono-latin-500-normal.woff2")}); }`;

const tmp = path.join(mediaDir, "frames");
await mkdir(tmp, { recursive: true });
const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM || undefined });
const page = await browser.newPage({ viewport: { width: 1280, height: 720 }, deviceScaleFactor: 1 });

for (const [name, t, kicker, title] of THUMBS) {
  const video = path.join(mediaDir, `video-${name}.mp4`);
  const frame = path.join(tmp, `thumb-${name}.jpg`);
  await run(FFMPEG, ["-y", "-loglevel", "error", "-ss", String(t), "-i", video, "-frames:v", "1", "-q:v", "2", frame]);
  const bg = "data:image/jpeg;base64," + (await readFile(frame)).toString("base64");
  const len = await duration(video);
  await page.setContent(`<!doctype html><style>${fonts}
    * { margin: 0; box-sizing: border-box; }
    body { width: 1280px; height: 720px; overflow: hidden; background: #0a0d12; color: #e6edf3; }
    #bg { position: absolute; inset: 0; background: url(${bg}) right center / cover; filter: saturate(.9); }
    #fade { position: absolute; inset: 0; background: linear-gradient(90deg, #0a0d12 34%, rgba(10,13,18,.85) 50%, rgba(10,13,18,.1) 74%); }
    #box { position: absolute; left: 64px; top: 0; bottom: 0; width: 600px; display: flex; flex-direction: column; justify-content: center; }
    #k { font: 500 26px M, monospace; color: #5eb8ff; letter-spacing: .12em; text-transform: uppercase; }
    h1 { font: 700 96px/1 G, sans-serif; letter-spacing: -.025em; margin-top: 22px; }
    #play { position: absolute; left: 890px; top: 290px; width: 140px; height: 140px; border-radius: 50%;
      background: rgba(94,184,255,.95); box-shadow: 0 0 0 12px rgba(94,184,255,.22), 0 12px 40px rgba(0,0,0,.5); }
    #play::after { content: ""; position: absolute; left: 56px; top: 42px; border-style: solid;
      border-width: 28px 0 28px 44px; border-color: transparent transparent transparent #0a121e; }
    #len { position: absolute; right: 28px; bottom: 26px; padding: 8px 16px; border-radius: 8px;
      background: rgba(10,13,18,.88); font: 500 30px M, monospace; letter-spacing: .04em; }
    #bar { position: absolute; left: 0; right: 0; bottom: 0; height: 5px; background: linear-gradient(90deg, #5eb8ff, #4ade9a); }
    </style>
    <div id="bg"></div><div id="fade"></div>
    <div id="box"><div id="k">${kicker}</div><h1>${title}</h1></div>
    <div id="play"></div><div id="len">${len}</div><div id="bar"></div>`);
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: path.join(mediaDir, `thumb-${name}.jpg`), type: "jpeg", quality: 86 });
  console.log(`thumb-${name}.jpg (${len})`);
}
await browser.close();
