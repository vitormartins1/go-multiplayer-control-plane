// render.mjs — renderiza o filme (film.html) quadro a quadro e gera o MP4 e as imagens paradas.
//
//   node docs/media/src/render.mjs                 # vídeo + imagens
//   node docs/media/src/render.mjs --stills        # só as imagens paradas (segundos)
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

// Imagens paradas para a documentação: [nome, instante do filme, selo "assista"?].
// GIF foi descartado: a paleta de 256 cores estraga o degradê escuro e cada
// recorte passava de 10 MB. Onde precisa de movimento, o link vai para o MP4.
const STILLS = [
  ["hero", 46, true],           // "como funciona" com o passo 3 (eventos) aceso; capa do README
  ["invasao-p2p", 92.4, false], // exemplo P2P com a conquista desbloqueada
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

// selo "assista ao vídeo" desenhado na própria página, com a fonte do filme
async function stills() {
  const secs = Math.round(duration);
  const len = secs >= 60 ? `${Math.floor(secs / 60)}min${String(secs % 60).padStart(2, "0")}` : `${secs} s`;
  for (const [name, t, badge] of STILLS) {
    await page.evaluate(([x, on, s]) => {
      window.__seek(x);
      document.getElementById("__badge")?.remove();
      if (!on) return;
      const b = document.createElement("div");
      b.id = "__badge";
      b.innerHTML = `<span class="i">▶</span>Assista ao vídeo · ${s}`;
      b.style.cssText = "position:absolute;top:44px;right:60px;z-index:99;display:flex;align-items:center;gap:16px;" +
        "padding:14px 28px 14px 16px;border:3px solid #5eb8ff;border-radius:40px;background:rgba(10,18,30,.9);" +
        "color:#e8f0fa;font:600 30px 'Space Grotesk',sans-serif;";
      b.querySelector(".i").style.cssText = "display:grid;place-items:center;width:38px;height:38px;border-radius:50%;" +
        "background:#5eb8ff;color:#0a121e;font-size:16px;padding-left:3px;box-sizing:border-box;";
      document.body.appendChild(b);
    }, [t, badge, len]);
    // 1600 px de largura em JPEG: nítido no README e ~100 KB
    const png = await page.screenshot({ type: "png" });
    await run(["-y", "-loglevel", "error", "-f", "image2pipe", "-i", "-", "-vf", "scale=1600:-1:flags=lanczos", "-q:v", "3",
      path.join(out, `${name}.jpg`)], png);
  }
}

// run executa o ffmpeg; input opcional vai pelo stdin
const run = (a, input) => new Promise((ok, ko) => {
  const p = spawn(FFMPEG, a, { stdio: [input ? "pipe" : "ignore", "inherit", "inherit"] });
  if (input) p.stdin.end(input);
  p.on("close", (c) => (c ? ko(c) : ok()));
});

const args = process.argv.slice(2);
if (args[0] === "--stills") {
  await stills();
  await browser.close(); server.close(); process.exit(0);
}
if (args[0] === "--frames") {
  await mkdir(path.join(out, "frames"), { recursive: true });
  for (const t of args[1].split(",").map(Number)) {
    await page.evaluate((x) => window.__seek(x), t);
    await page.screenshot({ path: path.join(out, "frames", `t${String(t).padStart(5, "0")}.png`) });
  }
  await browser.close(); server.close(); process.exit(0);
}

// MP4: PNG por stdin → H.264, yuv420p (toca em qualquer lugar), faststart (web).
// CRF 25 + tune animation: o filme é quase todo cor chapada; o texto das telas
// continua nítido e o arquivo fica em ~7 MB (CRF 20 dava 16 MB).
const mp4 = path.join(out, "go-multiplayer-control-plane.mp4");
const ff = spawn(FFMPEG, ["-y", "-f", "image2pipe", "-framerate", String(FPS), "-i", "-",
  "-c:v", "libx264", "-preset", "slow", "-crf", "25", "-tune", "animation", "-pix_fmt", "yuv420p", "-movflags", "+faststart", mp4], { stdio: ["pipe", "inherit", "inherit"] });
const total = Math.round(duration * FPS);
for (let i = 0; i < total; i++) {
  await page.evaluate((x) => window.__seek(x), i / FPS);
  const buf = await page.screenshot({ type: "png" });
  if (!ff.stdin.write(buf)) await new Promise((r) => ff.stdin.once("drain", r));
  if (i % 150 === 0) process.stdout.write(`quadro ${i}/${total}\n`);
}
ff.stdin.end();
await new Promise((r) => ff.on("close", r));
await stills();
await browser.close(); server.close();
console.log("pronto:", mp4);
