// Vídeo 1 — "Do zero ao jogo online": registrar um jogo com um modo P2P e ver o
// primeiro mundo abrir no console, sem deploy.
//
//   node docs/media/src/record/01-cadastro.mjs
//
// Pré-requisitos: a stack da demo no ar (console em CONSOLE_URL, serviços e
// grpcurl para o stage.sh) e nenhum jogo com o slug de GAME_NAME.
import { execFile, spawn } from "node:child_process";
import path from "node:path";
import { promisify } from "node:util";
import { CONSOLE, concat, encodeFrames, mediaDir, renderCard, startRecording } from "./lib.mjs";

const here = path.dirname(new URL(import.meta.url).pathname);
const GAME_NAME = process.env.GAME_NAME || "Mata Adentro";
const MODE_ID = "mundo-aberto";
const tmp = path.join(mediaDir, "frames");

const rec = await startRecording("01-cadastro");
const { page } = rec;
const drawer = page.locator(".mantine-Drawer-content");
const select = async (label, option) => {
  await rec.click(drawer.getByRole("textbox", { name: label }));
  await rec.wait(500);
  await rec.click(page.getByRole("option", { name: option }));
};

await page.goto(CONSOLE + "/games");
await page.getByText("Folclore Souls").first().waitFor();
await rec.begin();

await rec.caption("Vídeo 1", "Um estúdio registra um jogo novo no console.");
await rec.wait(1800);
await rec.click(page.getByRole("button", { name: "Registrar jogo" }));
await rec.wait(700);

await rec.caption("Identidade", "O slug sai do nome, e a capa é gerada na hora a partir dele.", { left: true });
await rec.type(drawer.getByRole("textbox", { name: "Nome", exact: true }), GAME_NAME, { delay: 90 });
await rec.wait(1400);

await rec.caption("Modo", "Cada jogo declara modos. O modo diz a forma do multiplayer.", { left: true });
await rec.type(drawer.getByRole("textbox", { name: "ID do modo" }), MODE_ID);
await rec.type(drawer.getByRole("textbox", { name: "Nome de exibição" }), "Mundo aberto");
await select("Arquétipo", /Host World Drop-in/);
await rec.wait(600);
await rec.caption("Topologia", "Mundo do host com invasões, hospedado pelo próprio jogador via Steam: nenhum servidor para pagar.", { left: true });
await select("Topologia", /P2P/);
await rec.wait(2200);

await rec.caption("Parâmetros", "Papéis, vagas e regras de invasão são parâmetros do modo, não código.", { left: true });
await rec.moveTo(drawer.getByText("Slots de invasor"));
await rec.wait(1200);
await rec.moveTo(drawer.getByText("Invasão automática"));
await rec.wait(1800);

await rec.click(drawer.getByRole("button", { name: "Registrar jogo" }));
await page.waitForURL(/\/games\/[0-9a-f-]{36}/);
const gameId = page.url().match(/games\/([0-9a-f-]{36})/)[1];
await page.getByRole("tab", { name: /Sessions/ }).waitFor();
await rec.caption("Pronto", "Registrado. Matchmaking, sessões e conquistas já leem este modo, sem deploy.");
await rec.wait(2600);

// os atores entram: um host abre o mundo e, segundos depois, um invasor entra
const stage = spawn(path.join(here, "stage.sh"), ["world", gameId, MODE_ID], {
  env: { ...process.env, INVADE_AFTER: "6" }, stdio: "inherit" });
const staged = new Promise((r) => stage.on("close", r)); // pode fechar antes do fim da gravação
await rec.click(page.getByRole("tab", { name: /Sessions/ }));
await rec.caption("Ao vivo", "Um jogador abre o primeiro mundo…");
const row = page.getByRole("row").filter({ hasText: /steam:/ }).first();
await row.waitFor({ timeout: 20000 });
await rec.wait(900);
await rec.caption("Ao vivo", "…e outro invade. Hospedado pelo host (P2P), com sinal de vida em tempo real.");
await row.getByText(/INVASOR 1\/1/i).waitFor({ timeout: 25000 });
await rec.wait(1200);
await rec.click(row, { dx: -200 });
await rec.wait(900);
await rec.caption("Ao vivo", "O mundo por dentro: o host, as vagas de co-op e o invasor que acabou de entrar.");
await rec.reveal(page.getByText(/^matinta_/).first(), "center", true);
await rec.wait(2600);
await rec.caption("Confiança", "No P2P o jogo do host reporta os eventos, e o console deixa isso explícito.");
const note = page.getByText(/Mundo P2P:/).first();
await rec.reveal(note, "center", true);
await rec.highlight(note);
await rec.wait(3200);
await rec.highlight(null);
await rec.caption(null);
await rec.wait(600);
await staged;

const frames = await rec.end();
const body = path.join(tmp, "01-body.mp4");
await encodeFrames(frames, body);
await renderCard({ kicker: "Vídeo 1 · Cadastro de jogo", title: "Do zero ao jogo online",
  sub: "Registrar um jogo, declarar o modo multiplayer e ver o primeiro mundo abrir, em menos de um minuto." }, 4, path.join(tmp, "01-intro.mp4"));
await renderCard({ kicker: "Sem deploy", title: "Nenhuma linha de código<br/>por jogo.",
  sub: "Um formulário configurou matchmaking, sessões, P2P e conquistas. O resto do backend leu o modo sozinho." }, 4.5, path.join(tmp, "01-outro.mp4"));
await concat([path.join(tmp, "01-intro.mp4"), body, path.join(tmp, "01-outro.mp4")], path.join(mediaDir, "video-01-cadastro.mp4"));
await promisify(execFile)(path.join(here, "stage.sh"), ["stop"]);
console.log("pronto: docs/media/video-01-cadastro.mp4 (jogo " + gameId + ")");
