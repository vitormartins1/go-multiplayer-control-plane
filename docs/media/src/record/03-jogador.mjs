// Vídeo 3 — "Visão 360 do jogador": o suporte abre um jogador e vê conquistas,
// histórico de mundos, dados (inventário, stats) e o save na nuvem.
//
//   node docs/media/src/record/03-jogador.mjs
//
// O jogador é o invasor do último mundo do stage.sh (o do vídeo 2), que já tem
// a conquista e o histórico; o `stage.sh seed` dá a ele inventário, stats e save.
import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";
import { CONSOLE, concat, encodeFrames, mediaDir, renderCard, startRecording } from "./lib.mjs";

const here = path.dirname(new URL(import.meta.url).pathname);
const tmp = path.join(mediaDir, "frames");
await promisify(execFile)(path.join(here, "stage.sh"), ["seed"]);
const state = Object.fromEntries((await readFile(process.env.STATE || "/tmp/stage-world.env", "utf8"))
  .trim().split(/\s+/).map((kv) => kv.split("=")));
const handle = state.invader_handle;

const rec = await startRecording("03-jogador");
const { page } = rec;
await page.goto(CONSOLE + "/players");
await page.getByPlaceholder("Filtrar por handle…").waitFor();
await page.getByRole("row").filter({ hasText: handle }).waitFor();
await rec.begin();

await rec.caption("Vídeo 3", "Chega um chamado no suporte: “ganhei a conquista, mas não apareceu”.");
await rec.wait(2400);
await rec.type(page.getByPlaceholder("Filtrar por handle…"), handle, { delay: 80 });
await rec.wait(600);
await rec.click(page.getByRole("row").filter({ hasText: handle }).first());
await page.getByRole("tab", { name: "Achievements" }).waitFor();
await rec.wait(1200);

await rec.caption("Conquistas", "Tudo sobre o jogador numa tela, por jogo. A conquista está lá, com a data do desbloqueio.");
await rec.wait(3400);

await rec.click(page.getByRole("tab", { name: "History" }));
await rec.caption("Histórico", "Cada mundo em que ele entrou: o papel, o desfecho e quanto tempo durou.");
await rec.wait(3400);

await rec.click(page.getByRole("tab", { name: "Player data" }));
await rec.caption("Dados", "O que o jogo grava para ele: inventário e stats, em JSON e já lidos como tabela.");
await rec.wait(2400);
await rec.moveTo(page.getByText("pena_de_uirapuru").last());
await rec.wait(1400);
await rec.reveal(page.getByText(/invasoes_vencidas/i).last(), "center", true);
await rec.wait(1800);

await rec.click(page.getByRole("tab", { name: "Cloud saves" }));
await rec.caption("Save", "E o último save na nuvem: onde ele parou e com o que estava equipado.");
await rec.wait(3600);
await rec.caption(null);
await rec.wait(500);

const frames = await rec.end();
const body = path.join(tmp, "03-body.mp4");
await encodeFrames(frames, body);
await renderCard({ kicker: "Vídeo 3 · Jogador", title: "Visão 360 do jogador",
  sub: "Conquistas, histórico de mundos, dados e save na nuvem, na mesma tela: o que o suporte e o live-ops precisam." }, 4, path.join(tmp, "03-intro.mp4"));
await renderCard({ kicker: "Suporte sem banco de dados", title: "Uma tela, não quatro<br/>consoles.",
  sub: "Sem abrir o banco de um serviço e o S3 de outro: o console junta o que cada serviço sabe do jogador." }, 4.5, path.join(tmp, "03-outro.mp4"));
await concat([path.join(tmp, "03-intro.mp4"), body, path.join(tmp, "03-outro.mp4")], path.join(mediaDir, "video-03-jogador.mp4"));
console.log("pronto: docs/media/video-03-jogador.mp4 (" + handle + ")");
