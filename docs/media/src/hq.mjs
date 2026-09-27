// hq.mjs: versões em alta qualidade dos quatro vídeos, para o YouTube (sem o teto
// de 10 MB dos anexos do GitHub). Saem em docs/media/hq/, fora do git.
//
//   CRF=16 node docs/media/src/hq.mjs            # os três de feature, dos quadros já gravados
//   CRF=16 node docs/media/src/hq.mjs --film     # e o filme, renderizado de novo (~20 min)
//   NARRATE=1 node docs/media/src/hq.mjs 03-jogador   # só os vídeos citados; narrado se a gravação foi narrada
//
// Os vídeos de feature não são regravados: a gravação deixa os quadros e a lista
// com o instante de cada um (docs/media/frames/<vídeo>/list.ffconcat), e aqui eles
// só são codificados de novo com menos compressão.
import { execFile } from "node:child_process";
import { access, mkdir } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";

process.env.CRF ||= "16"; // antes de importar o lib, que lê o CRF
const { mediaDir, encodeList } = await import("./record/lib.mjs");
const { CARDS, finish } = await import("./record/cards.mjs");

const hq = path.join(mediaDir, "hq");
await mkdir(hq, { recursive: true });

const only = process.argv.slice(2).filter((a) => !a.startsWith("--"));
for (const name of Object.keys(CARDS).filter((n) => only.length === 0 || only.includes(n))) {
  const list = path.join(mediaDir, "frames", name, "list.ffconcat");
  try {
    await access(list);
  } catch {
    console.log(`${name}: sem quadros gravados (rode o roteiro de gravação antes); pulando`);
    continue;
  }
  const body = path.join(mediaDir, "frames", `${name.slice(0, 2)}-body-hq.mp4`);
  await encodeList(list, body);
  console.log("pronto:", await finish(name, body, hq));
}

if (process.argv.includes("--film")) {
  await promisify(execFile)("node", [path.join(path.dirname(new URL(import.meta.url).pathname), "render.mjs")], {
    env: { ...process.env, FILM_OUT: path.join(hq, "go-multiplayer-control-plane.mp4") },
    maxBuffer: 64 << 20,
  });
  console.log("pronto:", path.join(hq, "go-multiplayer-control-plane.mp4"));
}
