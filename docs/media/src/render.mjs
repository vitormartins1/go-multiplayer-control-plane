// render.mjs — renderiza o filme (film.html) quadro a quadro e gera o MP4 e os GIFs.
//
//   node docs/media/src/render.mjs                 # vídeo + GIFs
//   node docs/media/src/render.mjs --frames 12,30  # só quadros soltos (PNG) para revisão
//
// Requisitos: playwright (vem com o admin-ui) e um ffmpeg no PATH ou em FFMPEG.
// O Chromium é o do Playwright (PLAYWRIGHT_CHROMIUM para outro caminho).
import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { readFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const out = path.resolve(here, "..");
const repo = path.resolve(here, "../../..");
const { chromium } = await import(path.join(repo, "services/admin-ui/node_modules/playwright/index.mjs"));
const FFMPEG = process.env.FFMPEG || "ffmpeg";
const FPS = 30;

// GIFs: [nome, início, fim] em segundos do filme (ver o roteiro em film.html)
const GIFS = [
  ["arquitetura", 12.6, 28.2],
  ["invasao-p2p", 28.6, 46.3],
  ["console-rooms", 46.6, 60.0],
  ["console-conquistas-analytics", 64.0, 71.0],
];

// servidor estático: o film.html busca shots/boxes.json (fetch não funciona em file://)
const types = { ".html": "text/html", ".jpg": "image/jpeg", ".json": "application/json", ".woff2": "font/woff2" };
const server = createServer(async (req, res) => {
  try {
    const f = path.join(here, decodeURIComponent(new URL(req.url, "http://x").pathname));
    const body = await readFile(f); // lê antes: 404 ainda pode ser enviado se falhar
    res.writeHead(200, { "content-type": types[path.extname(f)] || "application/octet-stream" });
    res.end(body);
  } catch { res.writeHead(404); res.end(); }
}).listen(0);
const url = `http://127.0.0.1:${server.address().port}/film.html`;

const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM || undefined });
const page = await browser.newPage({ viewport: { width: 1920, height: 1080 }, deviceScaleFactor: 1 });
await page.goto(url);
await page.evaluate(() => window.__ready);
const duration = await page.evaluate(() => window.__DURATION);

const args = process.argv.slice(2);
if (args[0] === "--frames") {
  await mkdir(path.join(out, "frames"), { recursive: true });
  for (const t of args[1].split(",").map(Number)) {
    await page.evaluate((x) => window.__seek(x), t);
    await page.screenshot({ path: path.join(out, "frames", `t${String(t).padStart(5, "0")}.png`) });
  }
  await browser.close(); server.close(); process.exit(0);
}

// MP4 master: PNG por stdin → H.264, yuv420p (toca em qualquer lugar), faststart (web)
const mp4 = path.join(out, "go-multiplayer-control-plane.mp4");
const ff = spawn(FFMPEG, ["-y", "-f", "image2pipe", "-framerate", String(FPS), "-i", "-",
  "-c:v", "libx264", "-preset", "slow", "-crf", "20", "-pix_fmt", "yuv420p", "-movflags", "+faststart", mp4], { stdio: ["pipe", "inherit", "inherit"] });
const total = Math.round(duration * FPS);
for (let i = 0; i < total; i++) {
  await page.evaluate((x) => window.__seek(x), i / FPS);
  const buf = await page.screenshot({ type: "png" });
  if (!ff.stdin.write(buf)) await new Promise((r) => ff.stdin.once("drain", r));
  if (i % 150 === 0) process.stdout.write(`quadro ${i}/${total}\n`);
}
ff.stdin.end();
await new Promise((r) => ff.on("close", r));
await browser.close(); server.close();

// GIFs: recortes do MP4, 960 px, 15 fps, paleta própria por GIF (fica nítido e leve)
const run = (a) => new Promise((ok, ko) => spawn(FFMPEG, a, { stdio: "inherit" }).on("close", (c) => (c ? ko(c) : ok())));
for (const [name, a, b] of GIFS) {
  const vf = "fps=15,scale=960:-1:flags=lanczos";
  await run(["-y", "-ss", String(a), "-to", String(b), "-i", mp4, "-vf", `${vf},palettegen=stats_mode=diff:max_colors=160`, "/tmp/palette.png"]);
  await run(["-y", "-ss", String(a), "-to", String(b), "-i", mp4, "-i", "/tmp/palette.png",
    "-lavfi", `${vf}[x];[x][1:v]paletteuse=dither=sierra2_4a:diff_mode=rectangle`, path.join(out, `${name}.gif`)]);
}
console.log("pronto:", mp4);
