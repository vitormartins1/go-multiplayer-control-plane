// shrink.mjs: reencoda um MP4 em duas passadas para caber num teto de tamanho
// (padrão 9,5 MB), no lugar do original. O GitHub só aceita anexos de até 10 MB,
// e é o anexo (github.com/user-attachments/...) que toca embutido no README.
//
//   node docs/media/src/shrink.mjs docs/media/video-02-conquistas.mp4 [MB]
//
// Os vídeos são quase só tela parada e texto: numa taxa baixa o x264 gasta os bits
// onde a imagem muda, e o texto continua legível. Arquivo que já cabe fica como está.
import { execFile } from "node:child_process";
import { rename, rm, stat } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const run = promisify(execFile);
const FFMPEG = process.env.FFMPEG || "ffmpeg";
const [file, mbArg] = process.argv.slice(2);
if (!file) throw new Error("uso: shrink.mjs ARQUIVO.mp4 [MB]");
const limit = Number(mbArg || 9.5) * 1024 * 1024;

const { size } = await stat(file);
if (size <= limit) {
  console.log(`${file}: ${(size / 1048576).toFixed(1)} MB, já cabe`);
  process.exit(0);
}
// duração pelo cabeçalho do próprio ffmpeg (sem ffprobe)
const probe = await run(FFMPEG, ["-hide_banner", "-i", file]).catch((e) => e);
const m = /Duration: (\d+):(\d+):([\d.]+)/.exec(probe.stderr || "");
if (!m) throw new Error("duração não encontrada: " + file);
const secs = Number(m[1]) * 3600 + Number(m[2]) * 60 + Number(m[3]);
// 3% de folga para o contêiner; sem áudio, toda a taxa vai para o vídeo
const kbps = Math.floor((limit * 8 * 0.97) / secs / 1000);

const tmp = path.join(os.tmpdir(), "shrink-" + path.basename(file));
const log = path.join(os.tmpdir(), "shrink-2pass");
const base = ["-y", "-loglevel", "error", "-i", file, "-c:v", "libx264", "-preset", "slow", "-tune", "animation",
  "-b:v", `${kbps}k`, "-pix_fmt", "yuv420p", "-passlogfile", log];
await run(FFMPEG, [...base, "-pass", "1", "-an", "-f", "mp4", "/dev/null"]);
await run(FFMPEG, [...base, "-pass", "2", "-an", "-movflags", "+faststart", tmp]);
await rename(tmp, file);
for (const f of [`${log}-0.log`, `${log}-0.log.mbtree`]) await rm(f, { force: true });
console.log(`${file}: ${(size / 1048576).toFixed(1)} → ${((await stat(file)).size / 1048576).toFixed(1)} MB (${kbps} kb/s)`);
