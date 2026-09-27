// lib.mjs — grava o console real para os vídeos de feature.
//
// Diferente do filme (film.html, quadro a quadro), aqui o console roda de
// verdade sobre os dados da demo e a gravação é em tempo real: o Chromium
// manda os quadros pelo screencast do CDP (JPEG, com o instante de cada um) e
// o ffmpeg monta o vídeo respeitando esses instantes. Por cima do console, um
// cursor e legendas injetados na página (o headless não desenha cursor).
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
import { mkdir, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
export const repo = path.resolve(here, "../../../..");
export const mediaDir = path.resolve(here, "../..");
const { chromium } = await import(path.join(repo, "services/admin-ui/node_modules/playwright/index.mjs"));
export const FFMPEG = process.env.FFMPEG || "ffmpeg";
export const CONSOLE = process.env.CONSOLE_URL || "http://localhost:5173";
// CRF do x264 nos vídeos de feature: 22 para o repositório (depois o shrink.mjs
// encaixa em 10 MB); o hq.mjs usa um valor menor para a versão do YouTube.
const CRF = process.env.CRF || "22";

// Narração (NARRATE=1): cada legenda vira fala pelo narrate.py (Kokoro, voz
// pm_alex). A legenda só aparece depois que a fala anterior terminou, o say
// espera a fala inteira, e os instantes de cada fala ficam em
// frames/<vídeo>/narration.json para o finish (cards.mjs) mixar o áudio.
export const NARRATE = !!process.env.NARRATE;
const GAP_MS = 350; // respiro entre uma fala e a próxima
let narrator = null;

// speak devolve { file, dur } do áudio da fala (o narrate.py mantém cache por texto)
export function speak(text) {
  if (!narrator) {
    const proc = spawn(process.env.PYTHON || "python3", [path.join(here, "narrate.py")], { stdio: ["pipe", "pipe", "inherit"] });
    const pending = [];
    createInterface({ input: proc.stdout }).on("line", (line) => pending.shift()?.(JSON.parse(line)));
    narrator = { proc, pending };
  }
  return new Promise((ok) => {
    narrator.pending.push(ok);
    narrator.proc.stdin.write(JSON.stringify({ text }) + "\n");
  });
}

export function stopNarrator() {
  narrator?.proc.stdin.end();
  narrator = null;
}

// 1280×720 com DPR 1,5 = quadros de 1920×1080 com a interface em tamanho legível
const VIEW = { width: 1280, height: 720 };
const DPR = 1.5;

// Camada injetada em todo documento: cursor, anel de clique e legenda. As cores
// e fontes seguem o filme (docs/media/src/film.html).
const OVERLAY = `
(() => {
  if (window.__rec) return;
  const css = \`
  #__cur{position:fixed;left:0;top:0;width:22px;height:22px;z-index:2147483647;pointer-events:none;
    transform:translate(-100px,-100px);transition:transform var(--d,600ms) cubic-bezier(.45,.05,.2,1)}
  #__cur svg{filter:drop-shadow(0 2px 4px rgba(0,0,0,.6))}
  .__ring{position:fixed;width:34px;height:34px;margin:-17px 0 0 -17px;border-radius:50%;z-index:2147483646;
    pointer-events:none;border:2px solid #5eb8ff;animation:__ring .5s ease-out forwards}
  @keyframes __ring{from{transform:scale(.3);opacity:1}to{transform:scale(1.4);opacity:0}}
  #__cap{position:fixed;left:50%;bottom:28px;z-index:2147483645;pointer-events:none;max-width:980px;
    transform:translate(-50%,16px);opacity:0;transition:opacity .35s,transform .35s;
    padding:12px 20px 12px 18px;border-radius:12px;background:rgba(8,14,24,.92);border:1px solid rgba(94,184,255,.45);
    box-shadow:0 10px 40px rgba(0,0,0,.5);color:#e8f0fa;font:500 17px/1.35 'IBM Plex Sans',system-ui,sans-serif;
    display:flex;gap:12px;align-items:baseline}
  #__cap.on{opacity:1;transform:translate(-50%,0)}
  #__cap.left{left:24px;max-width:560px;transform:translate(0,16px)}
  #__cap.left.on{transform:translate(0,0)}
  #__cap b{color:#5eb8ff;font:600 12px 'IBM Plex Mono',monospace;letter-spacing:.08em;text-transform:uppercase;white-space:nowrap}
  #__hl{position:fixed;z-index:2147483644;pointer-events:none;border:2px solid #5eb8ff;border-radius:10px;
    box-shadow:0 0 0 9999px rgba(3,6,12,.45),0 0 24px rgba(94,184,255,.5);opacity:0;transition:all .45s ease}\`;
  const boot = () => {
    const st = document.createElement("style"); st.textContent = css; document.head.appendChild(st);
    const c = document.createElement("div"); c.id = "__cur";
    c.innerHTML = '<svg width="22" height="22" viewBox="0 0 22 22"><path d="M3 2l15 8.5-6.5 1.6L8.6 19z" fill="#fff" stroke="#0a121e" stroke-width="1.4" stroke-linejoin="round"/></svg>';
    const cap = document.createElement("div"); cap.id = "__cap";
    const hl = document.createElement("div"); hl.id = "__hl";
    document.body.append(c, cap, hl);
  };
  window.__rec = {
    move(x, y, ms) { const c = document.getElementById("__cur"); c.style.setProperty("--d", ms + "ms"); c.style.transform = \`translate(\${x}px,\${y}px)\`; },
    ring(x, y) { const r = document.createElement("div"); r.className = "__ring"; r.style.left = x + "px"; r.style.top = y + "px"; document.body.appendChild(r); setTimeout(() => r.remove(), 600); },
    caption(kicker, text, left, width) { const c = document.getElementById("__cap"); if (!text) { c.classList.remove("on"); return; }
      c.style.maxWidth = width ? width + "px" : "";
      c.innerHTML = (kicker ? '<b>' + kicker + '</b>' : '') + '<span>' + text + '</span>';
      c.classList.toggle("left", !!left); c.classList.add("on"); },
    highlight(r) { const h = document.getElementById("__hl"); if (!r) { h.style.opacity = 0; return; }
      Object.assign(h.style, { left: r.x - 6 + "px", top: r.y - 6 + "px", width: r.width + 12 + "px", height: r.height + 12 + "px", opacity: 1 }); },
  };
  if (document.body) boot(); else document.addEventListener("DOMContentLoaded", boot);
})();`;

export async function startRecording(name) {
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM || undefined });
  const ctx = await browser.newContext({ viewport: VIEW, deviceScaleFactor: DPR, colorScheme: "dark", locale: "pt-BR" });
  await ctx.addInitScript(OVERLAY);
  const page = await ctx.newPage();
  const dir = path.join(mediaDir, "frames", name);
  await rm(dir, { recursive: true, force: true });
  await mkdir(dir, { recursive: true });

  const cdp = await ctx.newCDPSession(page);
  const frames = []; // { file, t } — t em segundos (relógio do screencast)
  let recording = false;
  const writes = [];
  cdp.on("Page.screencastFrame", ({ data, metadata, sessionId }) => {
    cdp.send("Page.screencastFrameAck", { sessionId }).catch(() => {});
    if (!recording) return;
    const file = path.join(dir, `f${String(frames.length).padStart(5, "0")}.jpg`);
    frames.push({ file, t: metadata.timestamp });
    writes.push(writeFile(file, Buffer.from(data, "base64")));
  });

  let cur = { x: -100, y: -100 };
  const narration = []; // { t (relógio de parede, s), file, dur, text }
  let audioUntil = 0;
  const rec = {
    page,
    async begin() {
      await cdp.send("Page.startScreencast", { format: "jpeg", quality: 92, maxWidth: 1920, maxHeight: 1080 });
      recording = true;
      await rec.wait(300);
    },
    wait: (ms) => page.waitForTimeout(ms),
    // rola suave até o elemento ficar visível (o scroll instantâneo do
    // Playwright dá um salto feio no vídeo)
    async reveal(loc, block = "center", force = false) {
      await loc.waitFor({ state: "visible" });
      const inView = !force && await loc.evaluate((el) => {
        const r = el.getBoundingClientRect();
        return r.top >= 60 && r.bottom <= window.innerHeight - 90;
      });
      if (inView) return;
      await loc.evaluate((el, block) => el.scrollIntoView({ behavior: "smooth", block }), block);
      await rec.settle(loc);
    },
    // espera o elemento parar de se mexer (scroll suave, drawer abrindo) antes
    // de medir: senão o destaque sai deslocado
    async settle(loc) {
      let last = null;
      for (let i = 0; i < 25; i++) {
        await rec.wait(120);
        const b = await loc.boundingBox();
        if (b && last && Math.abs(b.y - last.y) < 0.5 && Math.abs(b.x - last.x) < 0.5) return;
        last = b;
      }
    },
    // move o cursor até o centro do elemento, com duração proporcional à distância
    async moveTo(loc, { dx = 0, dy = 0 } = {}) {
      await rec.reveal(loc);
      const b = await loc.boundingBox();
      if (!b) throw new Error("moveTo: elemento sem caixa");
      const x = b.x + b.width / 2 + dx, y = b.y + b.height / 2 + dy;
      const dist = Math.hypot(x - cur.x, y - cur.y);
      const ms = cur.x < 0 ? 0 : Math.round(Math.min(900, Math.max(350, dist * 1.1)));
      await page.evaluate(([x, y, ms]) => window.__rec.move(x, y, ms), [x, y, ms]);
      await page.mouse.move(x, y);
      cur = { x, y };
      await rec.wait(ms + 120);
      return { x, y };
    },
    async click(loc, opts) {
      const { x, y } = await rec.moveTo(loc, opts);
      await page.evaluate(([x, y]) => window.__rec.ring(x, y), [x, y]);
      await page.mouse.click(x, y);
      await rec.wait(250);
    },
    // digita como gente: clique no campo e letra a letra
    async type(loc, text, { delay = 55 } = {}) {
      await rec.click(loc);
      await page.keyboard.type(text, { delay });
      await rec.wait(250);
    },
    // legenda embaixo; left=true quando um drawer ocupa a direita da tela
    // arrasta a partir de (x, y) com o cursor visível (pan do canvas)
    async drag(x, y, dx, dy) {
      await page.evaluate(([x, y]) => window.__rec.move(x, y, 450), [x, y]);
      await page.mouse.move(x, y);
      cur = { x, y };
      await rec.wait(550);
      await page.mouse.down();
      await page.evaluate(([x, y]) => window.__rec.move(x, y, 700), [x + dx, y + dy]);
      await page.mouse.move(x + dx, y + dy, { steps: 20 });
      await page.mouse.up();
      cur = { x: x + dx, y: y + dy };
      await rec.wait(750);
    },
    async caption(kicker, text, { left = false, width = 0 } = {}) {
      if (NARRATE) {
        // a legenda (e a fala) nova espera a fala anterior terminar
        const rest = audioUntil - Date.now();
        if (rest > 0) await rec.wait(rest);
        if (text) {
          const a = await speak(text.replace(/<[^>]+>/g, " "));
          await page.evaluate(([k, t, l, w]) => window.__rec.caption(k, t, l, w), [kicker, text, left, width]);
          narration.push({ t: Date.now() / 1000, file: a.file, dur: a.dur, text: text.replace(/<[^>]+>/g, " ") });
          audioUntil = Date.now() + a.dur * 1000 + GAP_MS;
          return;
        }
      }
      await page.evaluate(([k, t, l, w]) => window.__rec.caption(k, t, l, w), [kicker, text, left, width]);
    },
    // say mostra a legenda e espera o tempo de leitura (~14 caracteres por
    // segundo, entre 3,2 s e 8 s): quem não conhece o sistema precisa ler.
    async say(kicker, text, opts) {
      await rec.caption(kicker, text, opts);
      await rec.wait(NARRATE ? Math.max(readMs(text), audioUntil - Date.now()) : readMs(text));
    },
    // spot destaca um elemento (escurece o resto), explica e segura; o destaque
    // sai no fim, a legenda fica até a próxima
    async spot(loc, kicker, text, opts = {}) {
      await rec.reveal(loc.first(), "center", !!opts.force);
      await rec.highlight(loc);
      await rec.say(kicker, text, opts);
      if (!opts.keep) await rec.highlight(null);
    },
    // highlight aceita um locator com vários elementos (uma coluna de nós do
    // canvas, por exemplo): o destaque cobre a caixa que envolve todos
    async highlight(loc) {
      if (loc) await rec.settle(loc.first());
      let b = null;
      if (loc) {
        const boxes = (await Promise.all((await loc.all()).map((l) => l.boundingBox()))).filter(Boolean);
        if (boxes.length) {
          const x = Math.min(...boxes.map((r) => r.x)), y = Math.min(...boxes.map((r) => r.y));
          b = { x, y, width: Math.max(...boxes.map((r) => r.x + r.width)) - x, height: Math.max(...boxes.map((r) => r.y + r.height)) - y };
        }
      }
      await page.evaluate((r) => window.__rec.highlight(r), b);
    },
    async end() {
      if (NARRATE) {
        const rest = audioUntil - Date.now(); // a última fala termina antes de parar
        if (rest > 0) await rec.wait(rest);
      }
      recording = false;
      await cdp.send("Page.stopScreencast");
      await Promise.all(writes);
      await browser.close();
      if (NARRATE) {
        // t0 = instante do primeiro quadro: o corpo do vídeo começa nele
        // frames: o instante de cada quadro. A legenda só aparece no primeiro quadro
        // depois do pedido (até ~0,6 s depois); o finish ancora a fala nele.
        await writeFile(path.join(dir, "narration.json"),
          JSON.stringify({ t0: frames[0].t, frames: frames.map((f) => f.t), events: narration }));
        stopNarrator();
      }
      return frames;
    },
  };
  return rec;
}

export const readMs = (text) => Math.min(8000, Math.max(3200, (text || "").replace(/<[^>]+>/g, "").length * 70));

const run = (a) => new Promise((ok, ko) => spawn(FFMPEG, a, { stdio: ["ignore", "inherit", "inherit"] }).on("close", (c) => (c ? ko(new Error("ffmpeg " + c)) : ok())));

// encode monta o MP4 com os instantes reais dos quadros (concat demuxer com
// duração por quadro) e normaliza para 30 fps constantes.
export async function encodeFrames(frames, out) {
  if (frames.length < 2) throw new Error("gravação vazia");
  const lines = ["ffconcat version 1.0"];
  for (let i = 0; i < frames.length; i++) {
    const d = i + 1 < frames.length ? frames[i + 1].t - frames[i].t : 0.5;
    lines.push(`file '${frames[i].file}'`, `duration ${Math.max(d, 0.001).toFixed(4)}`);
  }
  lines.push(`file '${frames.at(-1).file}'`); // o concat ignora a duração do último item
  const list = path.join(path.dirname(frames[0].file), "list.ffconcat");
  await writeFile(list, lines.join("\n"));
  await encodeList(list, out);
}

// encodeList monta o MP4 a partir da lista de quadros que a gravação deixou
// (docs/media/frames/<vídeo>/list.ffconcat): é o que o hq.mjs reaproveita.
export async function encodeList(list, out) {
  await run(["-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", list,
    "-vf", "fps=30,scale=1920:1080:flags=lanczos,format=yuv420p",
    "-c:v", "libx264", "-preset", "slow", "-crf", CRF, "-tune", "animation", "-movflags", "+faststart", out]);
}

// card renderiza uma cartela (card.html) quadro a quadro, como o filme
export async function renderCard(params, seconds, out) {
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM || undefined });
  const page = await browser.newPage({ viewport: { width: 1920, height: 1080 } });
  const q = new URLSearchParams({ ...params, dur: String(seconds) });
  await page.goto("file://" + path.join(here, "card.html") + "?" + q);
  await page.evaluate(() => document.fonts.ready);
  const ff = spawn(FFMPEG, ["-y", "-loglevel", "error", "-f", "image2pipe", "-framerate", "30", "-i", "-",
    "-c:v", "libx264", "-preset", "slow", "-crf", CRF, "-tune", "animation", "-pix_fmt", "yuv420p", out], { stdio: ["pipe", "inherit", "inherit"] });
  for (let i = 0; i < seconds * 30; i++) {
    await page.evaluate((t) => window.__seek(t), i / 30);
    const buf = await page.screenshot({ type: "png" });
    if (!ff.stdin.write(buf)) await new Promise((r) => ff.stdin.once("drain", r));
  }
  ff.stdin.end();
  await new Promise((r) => ff.on("close", r));
  await browser.close();
}

// mediaDuration lê a duração (s) do cabeçalho que o ffmpeg imprime (sem ffprobe)
export async function mediaDuration(file) {
  const err = await new Promise((ok) => {
    const p = spawn(FFMPEG, ["-hide_banner", "-i", file], { stdio: ["ignore", "ignore", "pipe"] });
    let buf = "";
    p.stderr.on("data", (d) => (buf += d));
    p.on("close", () => ok(buf));
  });
  const m = /Duration: (\d+):(\d+):([\d.]+)/.exec(err);
  if (!m) throw new Error("duração não encontrada: " + file);
  return Number(m[1]) * 3600 + Number(m[2]) * 60 + Number(m[3]);
}

// mixNarration põe as falas no vídeo: cada { at (s), file } entra no seu instante,
// o volume é normalizado para a faixa de voz do YouTube (-16 LUFS) e o vídeo é
// copiado sem reencodar.
export async function mixNarration(video, clips, out) {
  const dur = await mediaDuration(video);
  const inputs = clips.flatMap((c) => ["-i", c.file]);
  const chains = clips.map((c, i) => `[${i + 1}:a]adelay=delays=${Math.round(c.at * 1000)}:all=1[a${i}]`);
  // apad até a duração exata do vídeo: apad sem fim + -shortest nunca termina
  // quando o vídeo é copiado (o ffmpeg fica gerando silêncio)
  const mix = clips.map((_, i) => `[a${i}]`).join("") +
    `amix=inputs=${clips.length}:normalize=0:dropout_transition=0,loudnorm=I=-16:TP=-1.5:LRA=11,aresample=48000,` +
    `apad=whole_dur=${dur.toFixed(3)},atrim=0:${dur.toFixed(3)}[aout]`;
  await run(["-y", "-loglevel", "error", "-i", video, ...inputs, "-filter_complex", [...chains, mix].join(";"),
    "-map", "0:v", "-map", "[aout]", "-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", out]);
}

// concat junta cartela + gravação + cartela num MP4 só (reencoda: timebases diferem)
export async function concat(parts, out) {
  const list = out + ".txt";
  await writeFile(list, parts.map((p) => `file '${p}'`).join("\n"));
  await run(["-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", list, "-c:v", "libx264", "-preset", "slow",
    "-crf", CRF, "-tune", "animation", "-pix_fmt", "yuv420p", "-r", "30", "-movflags", "+faststart", out]);
  await rm(list);
}

// srt: uma legenda por fala, no instante em que ela toca
export function srt(clips) {
  const ts = (s) => {
    const ms = Math.max(0, Math.round(s * 1000));
    const p = (n, w = 2) => String(n).padStart(w, "0");
    return `${p(Math.floor(ms / 3600000))}:${p(Math.floor(ms / 60000) % 60)}:${p(Math.floor(ms / 1000) % 60)},${p(ms % 1000, 3)}`;
  };
  return clips.map((c, i) => `${i + 1}\n${ts(c.at)} --> ${ts(c.at + c.dur)}\n${c.text.replace(/\s+/g, " ").trim()}\n`).join("\n");
}
