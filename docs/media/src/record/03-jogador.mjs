// Vídeo 3 — "Visão 360 do jogador": um tour de produto pela tela do jogador,
// aba por aba, a partir de quem está online agora num jogo.
//
//   node docs/media/src/record/03-jogador.mjs
//
// Usa o Folclore Souls e o jogador "saci" da demo, preparados antes da gravação
// (ver docs/media/README.md): conquistas com uma em progresso, histórico com um
// mundo que terminou em "Host morto" e outro fechado por falta de heartbeat,
// inventário, stats e um save. Durante a gravação o stage.sh abre um mundo novo
// com o saci de host, para ele aparecer online.
import { spawn } from "node:child_process";
import path from "node:path";
import { CONSOLE, concat, encodeFrames, mediaDir, renderCard, startRecording } from "./lib.mjs";

const here = path.dirname(new URL(import.meta.url).pathname);
const GAME_NAME = process.env.GAME_NAME || "Folclore Souls";
const PLAYER = process.env.PLAYER || "saci";
const tmp = path.join(mediaDir, "frames");

const games = await (await fetch(CONSOLE + "/api/games")).json();
const game = games.find((g) => g.name === GAME_NAME);
if (!game) throw new Error(`jogo "${GAME_NAME}" não encontrado`);
const stage = (...args) => new Promise((ok, ko) =>
  spawn(path.join(here, "stage.sh"), args, {
    env: { ...process.env, STATE: "/tmp/stage-v3.env", HOST: PLAYER, INVADER: "curupira", INVADE_AFTER: "2" }, stdio: "inherit",
  }).on("close", (c) => (c ? ko(new Error("stage " + args[0])) : ok())));

// o saci abre um mundo antes de a câmera ligar: ele precisa estar online
await stage("world", game.id, "default");

const rec = await startRecording("03-jogador");
const { page } = rec;
const tab = (name) => page.getByRole("tab", { name: new RegExp("^" + name) });
await page.goto(`${CONSOLE}/games/${game.id}`);
await tab("Players").waitFor();
await rec.begin();

// --- 1. quem está online no jogo -----------------------------------------------
await rec.say("Vídeo 3", "A visão 360 do jogador: tudo o que o backend sabe sobre uma pessoa, numa tela só.");
await rec.click(tab("Players"));
await rec.wait(1200);
const onlineRow = page.getByRole("row").filter({ hasText: PLAYER }).first();
await rec.spot(page.getByRole("row").filter({ hasText: /^Online/ }).first(), "Online",
  "A aba Players do jogo separa quem está numa sessão agora. O ponto verde no avatar indica online.");
await rec.spot(onlineRow, "Jogador", `O ${PLAYER} está online: ele é o host de um mundo aberto neste momento. Vamos abrir a visão dele.`);
await rec.click(onlineRow);
await tab("Achievements").waitFor();
await rec.wait(1400);

// --- 2. cabeçalho ------------------------------------------------------------------
const header = page.getByRole("heading", { name: PLAYER }).locator("xpath=ancestor::div[contains(@class,'mantine-Group-root')][2]");
await rec.spot(header, "Identidade", "O handle, o avatar e o id da conta, com um botão para copiar.");
await rec.spot(page.getByRole("button", { name: GAME_NAME }).first(), "Por jogo",
  "Cada chip é um jogo em que ele já entrou. Tudo abaixo muda conforme o jogo escolhido.");

// --- 3. conquistas -----------------------------------------------------------------
await rec.spot(page.getByText("Progresso geral").locator("xpath=.."), "Conquistas",
  "O progresso geral no catálogo do jogo: quantas ele já tem e quanto falta.");
await rec.spot(page.getByRole("row").filter({ hasText: "Implacável" }).first(), "Desbloqueada",
  "Cada conquista mostra o progresso e, quando desbloqueada, a data e a hora.");
await rec.spot(page.getByRole("row").filter({ hasText: "Mão Pesada" }).first(), "Em progresso",
  "Mão Pesada soma abates entre partidas: 22 de 50. A barra avança a cada partida que ele joga.");
await rec.caption("Organizar", "Dá para ordenar por nome ou por progresso, e agrupar por estado.");
await rec.click(page.getByText("Progresso", { exact: true }).first());
await rec.wait(1400);
await rec.click(page.getByText("Por estado", { exact: true }));
await rec.wait(1200);
await rec.say("Por estado", "Conquistadas, em progresso e ainda não iniciadas, cada grupo numa seção.");

// --- 4. histórico --------------------------------------------------------------------
await rec.click(tab("History"));
await rec.wait(1400);
await rec.say("Histórico", "Cada mundo e partida de que ele participou, do mais recente para o mais antigo.");
await rec.spot(page.getByRole("row").filter({ hasText: "Host morto" }).first(), "Desfecho",
  "Aqui ele era o host e foi derrotado por um invasor: o papel, o resultado, o motivo do fim e a duração.");
const dropped = page.getByRole("row").filter({ hasText: "Host caiu" }).first();
if (await dropped.count()) {
  await rec.spot(dropped, "Queda", "Neste o jogo dele parou de mandar sinal de vida. O mundo P2P fechou sozinho, marcado como queda do host.");
}
await rec.spot(page.locator("table:visible tbody tr td:last-child").first(), "Participantes",
  "E com quem ele estava em cada sessão.");

// --- 5. dados do jogador -------------------------------------------------------------
await rec.click(tab("Player data"));
await rec.wait(1400);
await rec.say("Dados", "O que o jogo grava para ele, organizado por namespace e chave.");
await rec.spot(page.getByRole("row").filter({ hasText: "mochila" }).first().locator("td").nth(2), "JSON",
  "O valor bruto em JSON, do jeito que o jogo mandou: aqui, a mochila com ouro e itens.");
await rec.spot(page.getByRole("row").filter({ hasText: "mochila" }).first().locator("td").nth(3), "Leitura",
  "Ao lado, a mesma coisa lida como tabela, sem precisar entender JSON.");
await rec.spot(page.getByRole("row").filter({ hasText: "stats" }).first(), "Stats",
  "E as stats de progresso: nível, horas jogadas, chefes derrotados.", { force: true });

// --- 6. save na nuvem ------------------------------------------------------------------
await rec.click(tab("Cloud saves"));
await rec.wait(1400);
await rec.spot(page.locator("table:visible").first(), "Save",
  "O save na nuvem: tipo, tamanho e quando foi gravado pela última vez.");
await rec.spot(page.getByText("Leitura consolidada").locator("xpath=ancestor::div[contains(@class,'mantine-Card-root')][1]"), "Conteúdo",
  "E o conteúdo: onde ele parou, o nível e o que estava equipado.");

// --- 7. a sessão ativa -----------------------------------------------------------------
await rec.caption("Ao vivo", "E a sessão em que ele está agora? Rooms mostra tudo o que está aberto, em todos os jogos.");
await rec.click(page.getByRole("link", { name: "Rooms" }));
await rec.wait(1800);
const live = page.getByRole("row").filter({ hasText: /ao vivo/i }).filter({ hasText: GAME_NAME }).filter({ hasText: /steam:/ }).first();
await rec.spot(live, "Sessão ativa", "O mundo que ele abriu: ao vivo, hospedado por ele via P2P, com um invasor dentro.");
await rec.click(live, { dx: -200 });
await rec.wait(1000);
await rec.spot(page.getByText(/^matinta_|^curupira/).first().locator("xpath=ancestor::div[contains(@class,'mantine-Paper-root')][1]/.."), "Dentro do mundo",
  `Os papéis de cada um: ${PLAYER} como host, as vagas de co-op livres e o invasor.`, { force: true });
await rec.caption(null);
await rec.wait(600);
await stage("stop");

const frames = await rec.end();
const body = path.join(tmp, "03-body.mp4");
await encodeFrames(frames, body);
if (process.env.ONLY_BODY) process.exit(0); // ensaio: só a gravação, sem cartelas
await renderCard({ kicker: "Vídeo 3 · Jogador", title: "Visão 360 do jogador",
  sub: "Presença, conquistas, histórico, dados, save e a sessão ao vivo de cada jogador, na mesma tela." }, 4.5, path.join(tmp, "03-intro.mp4"));
await renderCard({ kicker: "Uma tela, todos os serviços", title: "Tudo sobre o jogador,<br/>num lugar só.",
  sub: "O console junta o que cada serviço sabe: contas, conquistas, sessões, dados e saves." }, 5, path.join(tmp, "03-outro.mp4"));
await concat([path.join(tmp, "03-intro.mp4"), body, path.join(tmp, "03-outro.mp4")], path.join(mediaDir, "video-03-jogador.mp4"));
console.log("pronto: docs/media/video-03-jogador.mp4");
