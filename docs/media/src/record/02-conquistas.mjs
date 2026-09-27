// Vídeo 2 — "Conquistas sem deploy": cadastrar uma conquista com nome na Steam,
// ver a regra que a alimenta e vê-la desbloquear quando um invasor derrota o host.
//
//   GAME_NAME="Mata Adentro" node docs/media/src/record/02-conquistas.mjs
//
// Usa o jogo do vídeo 1 (modo p2p "mundo-aberto", sem a conquista ainda).
import { spawn } from "node:child_process";
import path from "node:path";
import { CONSOLE, concat, encodeFrames, mediaDir, renderCard, startRecording } from "./lib.mjs";

const here = path.dirname(new URL(import.meta.url).pathname);
const GAME_NAME = process.env.GAME_NAME || "Mata Adentro";
const MODE_ID = process.env.MODE_ID || "mundo-aberto";
const tmp = path.join(mediaDir, "frames");

const games = await (await fetch(CONSOLE + "/api/games")).json();
const game = games.find((g) => g.name === GAME_NAME);
if (!game) throw new Error(`jogo "${GAME_NAME}" não encontrado`);
const stageRun = (...args) => new Promise((ok, ko) =>
  spawn(path.join(here, "stage.sh"), args, { env: { ...process.env, INVADE_AFTER: "3" }, stdio: "inherit" })
    .on("close", (c) => (c ? ko(new Error("stage " + args[0])) : ok())));

const rec = await startRecording("02-conquistas");
const { page } = rec;
await page.goto(`${CONSOLE}/games/${game.id}`);
await page.getByRole("button", { name: "Nova conquista" }).waitFor();
await rec.begin();

await rec.caption("Vídeo 2", "Conquistas são dados do jogo: um cadastro no console, não uma release.");
await rec.wait(2000);
await rec.click(page.getByRole("button", { name: "Nova conquista" }));
const modal = page.getByRole("dialog");
const side = { left: true, width: 370 }; // o modal ocupa o centro
await rec.caption("Catálogo", "Id, nome e descrição: o catálogo é do jogo, editado aqui.", side);
await rec.wait(500);
await rec.type(modal.getByRole("textbox", { name: "Id (slug)" }), "invasor_vitorioso", { delay: 45 });
await rec.type(modal.getByRole("textbox", { name: "Nome", exact: true }), "Invasor Vitorioso", { delay: 45 });
await rec.type(modal.getByRole("textbox", { name: "Descrição" }), "Derrotou o host do mundo invadido", { delay: 35 });

await rec.caption("Critério", "O critério é um tipo de evento. As opções vêm das regras do arquétipo do jogo.", side);
await rec.click(modal.getByRole("textbox", { name: "Event type" }));
await rec.wait(1600);
await rec.click(page.getByRole("option", { name: /invasion_won/ }));
await rec.caption("Steam", "Com o nome na Steam, o desbloqueio aparece também no perfil Steam do jogador.", side);
await rec.type(modal.getByRole("textbox", { name: /Nome na Steam/ }), "ACH_INVASOR_VITORIOSO", { delay: 45 });
await rec.wait(1000);
await rec.click(modal.getByRole("button", { name: "Adicionar" }));
await rec.wait(900);

const row = page.getByRole("row").filter({ hasText: "invasor_vitorioso" });
await rec.caption("Catálogo", "No catálogo, com o selo Steam. Nenhum serviço foi reiniciado.");
await rec.highlight(row);
await rec.wait(2400);
await rec.highlight(null);

// os atores: um mundo p2p abre, o invasor entra e o jogo do host reporta HOST_KILLED
const staged = stageRun("world", game.id, MODE_ID).then(() => stageRun("kill"));

await rec.caption("Regra", "De onde vem o evento: fato da partida → regra → tipo de evento → conquista.");
await rec.click(row.getByRole("button", { name: "Ver regras do evento" }));
await rec.wait(1000);
await rec.reveal(page.getByText(/regra global/i).first(), "center", true);
await rec.wait(600);
await rec.moveTo(page.getByText("role = invader").first());
await rec.wait(1400);
await rec.moveTo(page.getByText("invasion_won").last());
await rec.wait(1800);

await rec.caption("Ao vivo", "Num mundo P2P, um invasor derrota o host, e o jogo do host reporta HOST_KILLED…");
await staged;
await rec.wait(2600); // fan-out: SNS → SQS → consumer
await rec.click(row.getByRole("button", { name: "Ver progresso" }));
await rec.wait(900);
await rec.caption("Fan-out", "…e o evento vira conquista pelo fan-out, em segundos. Sem deploy, sem código novo.");
await rec.wait(3800);
await rec.caption(null);
await rec.wait(500);

const frames = await rec.end();
const body = path.join(tmp, "02-body.mp4");
await encodeFrames(frames, body);
await renderCard({ kicker: "Vídeo 2 · Conquistas", title: "Conquistas sem deploy",
  sub: "Cadastrar uma conquista, ver a regra que a alimenta e vê-la desbloquear quando um invasor vence." }, 4, path.join(tmp, "02-intro.mp4"));
await renderCard({ kicker: "Regras são dados", title: "Do evento à conquista<br/>em segundos.",
  sub: "Fatos da partida passam por regras editáveis no console e viram conquistas, também na Steam." }, 4.5, path.join(tmp, "02-outro.mp4"));
await concat([path.join(tmp, "02-intro.mp4"), body, path.join(tmp, "02-outro.mp4")], path.join(mediaDir, "video-02-conquistas.mp4"));
console.log("pronto: docs/media/video-02-conquistas.mp4");
