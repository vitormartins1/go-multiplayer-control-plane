// Vídeo 1 — "Do zero ao jogo online": registrar um jogo com um modo P2P, entender
// cada parte do formulário, ver o que o backend configurou sozinho (pipeline de
// dados e regras do arquétipo), o primeiro mundo abrir e ser invadido, e a partida
// terminar no histórico e no analytics, sem deploy.
//
//   node docs/media/src/record/01-cadastro.mjs
//
// Pré-requisitos: a stack da demo no ar (console em CONSOLE_URL, serviços e
// grpcurl para o stage.sh) e nenhum jogo com o slug de GAME_NAME (o slug de um
// jogo apagado continua reservado).
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
const section = (label) => drawer.locator(".mantine-Card-root").filter({ hasText: label }).first();
const side = { left: true }; // o drawer ocupa a metade direita
const select = async (label, option, kicker, text) => {
  await rec.click(drawer.getByRole("textbox", { name: label }));
  await rec.wait(600);
  await rec.say(kicker, text, side); // as opções ficam abertas enquanto a legenda explica
  await rec.click(page.getByRole("option", { name: option }));
};
const tab = (name) => page.getByRole("tab", { name: new RegExp("^" + name) });

await page.goto(CONSOLE + "/games");
await page.getByText("Folclore Souls").first().waitFor();
await rec.begin();

// --- 1. a lista de jogos ---------------------------------------------------------
await rec.say("Vídeo 1", "Do zero ao jogo online: registrar um jogo, definir como ele joga online e ver o primeiro mundo abrir.");
await rec.spot(page.locator(".game-card").first().locator("xpath=.."), "Jogos",
  "Cada jogo registrado vira um card. O mesmo backend atende vários jogos ao mesmo tempo.");
await rec.click(page.getByRole("button", { name: "Registrar jogo" }));
await rec.wait(800);
await rec.say("Registro", "Um formulário só: a identidade do jogo e o primeiro modo de multiplayer.", side);

// --- 2. identidade ---------------------------------------------------------------
await rec.type(drawer.getByRole("textbox", { name: "Nome", exact: true }), GAME_NAME, { delay: 110 });
await rec.wait(500);
await rec.spot(drawer.locator(".mantine-Card-root").first().locator("xpath=preceding-sibling::*[1]"), "Capa",
  "A capa é gerada na hora a partir do slug. Não precisa de arte para começar.", side);
await rec.spot(drawer.getByRole("textbox", { name: /^Slug/ }), "Slug",
  "O slug é a chave estável do jogo, criado a partir do nome. Não muda depois do registro.", side);

// --- 3. o modo -------------------------------------------------------------------
await rec.type(drawer.getByRole("textbox", { name: "ID do modo" }), MODE_ID, { delay: 70 });
await rec.type(drawer.getByRole("textbox", { name: "Nome de exibição" }), "Mundo aberto", { delay: 70 });
await rec.spot(section("Identidade do modo"), "Modo",
  "Um jogo pode ter vários modos. Cada um tem o id que o jogo usa e um nome para exibir.", side);
await select("Arquétipo", /Host World Drop-in/, "Arquétipo",
  "O arquétipo é a forma do multiplayer: arena por fila, mundo do host, battle royale ou missão co-op.");
await rec.wait(800);
await rec.say("Mundo do host", "Escolhemos mundo do host: outros jogadores entram no mundo de alguém, como aliados ou invasores. O formulário se adapta.", side);
await select("Topologia", /P2P/, "Topologia",
  "Quem hospeda o mundo: um servidor dedicado no Agones, ou o próprio jogo do host, via Steam (P2P).");
await rec.wait(800);

// --- 4. parâmetros do arquétipo ----------------------------------------------------
await rec.spot(section("Papéis e capacidade"), "Papéis",
  "Quem pode entrar no mundo de outro jogador, e quantos de cada ao mesmo tempo: 2 aliados e 1 invasor.", side);
await rec.spot(section("Critérios de matchmaking"), "Matchmaking",
  "Como o invasor encontra um mundo: pela faixa de nível, pelo tier de equipamento e, se quiser, com senha.", side);
await rec.spot(section("Comportamento"), "Invasão",
  "Com invasão automática, o matchmaking coloca invasores sem o host precisar chamar.", side);
const stats = section("Métricas").getByRole("textbox").first();
await rec.type(stats, "kills", { delay: 90 });
await page.keyboard.press("Enter");
await rec.type(stats, "boss_boitata", { delay: 70 });
await page.keyboard.press("Enter");
await rec.spot(section("Métricas"), "Métricas",
  "As stats que o jogo vai reportar. Elas alimentam conquistas e rankings.", side);

// --- 5. registrado ----------------------------------------------------------------
await rec.click(drawer.getByRole("button", { name: "Registrar jogo" }));
await page.waitForURL(/\/games\/[0-9a-f-]{36}/);
const gameId = page.url().match(/games\/([0-9a-f-]{36})/)[1];
await tab("Sessions").waitFor();
await rec.wait(1200);
await rec.spot(page.getByRole("heading", { name: GAME_NAME }).locator("xpath=.."), "Pronto",
  "Registrado. O selo mostra o modo e a topologia. Nenhum serviço foi reiniciado.");
await rec.spot(page.getByRole("tablist").first(), "O jogo",
  "Cada jogo tem suas abas: conquistas, regras, rankings, jogadores, sessões e analytics.");

// --- 5b. o que o backend configurou sozinho ------------------------------------------
await rec.click(tab("Configurações"));
await rec.wait(1400);
await rec.say("Sem código", "O que o backend fez com esse formulário? A aba Configurações mostra.");
await rec.spot(page.getByRole("row").filter({ hasText: MODE_ID }).first(), "Modo",
  "O modo que acabamos de criar: mundo do host, P2P, duas vagas de co-op, uma de invasor e as stats declaradas.");
await rec.spot(page.getByRole("row").filter({ hasText: "participation_event" }).first(), "Pipeline",
  "Cada evento da partida vira um envelope e segue para as conquistas e o analytics. Ninguém ligou isso à mão.", { force: true });
await rec.spot(page.getByRole("row").filter({ hasText: /regras/ }).first(), "Regras prontas",
  "E o jogo já nasceu com as regras do arquétipo: três regras, três eventos prontos para virar conquista.", { force: true });
await rec.click(tab("Event Rules"));
await rec.wait(1400);
await rec.spot(page.getByText("INVASION_WON").first().locator("xpath=ancestor::tbody[1]"), "Event Rules",
  "Aqui estão elas: entrar num mundo, vencer uma invasão, ajudar a derrotar um chefe. O vídeo 2 mostra como funcionam.");

// --- 6. frota: P2P não precisa ------------------------------------------------------
await rec.click(page.getByRole("link", { name: "Fleet" }));
await rec.wait(1500);
await rec.spot(page.getByRole("row").filter({ hasText: GAME_NAME }).first(), "Frota",
  "Na tela de frotas, o jogo novo aparece como P2P, sem frota: ele não precisa de servidor dedicado.");
await rec.click(page.getByRole("link", { name: "Games" }));
await rec.wait(1000);
await rec.click(page.locator(".game-card").filter({ hasText: GAME_NAME }));
await tab("Sessions").waitFor();

// --- 7. ao vivo -------------------------------------------------------------------
// os atores: um jogador abre um mundo e, segundos depois, outro invade
const stage = spawn(path.join(here, "stage.sh"), ["world", gameId, MODE_ID], {
  env: { ...process.env, INVADE_AFTER: "12" }, stdio: "inherit" });
const staged = new Promise((r) => stage.on("close", r)); // pode fechar antes do fim da gravação
await rec.click(tab("Sessions"));
await rec.caption("Ao vivo", "Agora um jogador abre o jogo e cria um mundo. Veja a lista de sessões…");
const row = page.getByRole("row").filter({ hasText: /steam:/ }).first();
await row.waitFor({ timeout: 30000 });
await rec.wait(700);
await rec.spot(row, "Mundo aberto", "Este é o mundo que ele acabou de abrir, com o modo, o estado e as vagas.", { keep: true });
await rec.spot(row.getByText(/steam:/), "Hospedagem", "P2P: quem hospeda é o jogo dele. O backend guarda só o SteamID para os outros conectarem.");
await rec.spot(row.getByText(/vivo ·/i), "Sinal de vida", "O jogo do host avisa a cada poucos segundos que o mundo continua vivo.");
await rec.caption("Invasão", "Agora outro jogador pede para invadir. Observe a vaga de invasor…");
await row.getByText(/invasor 1\/1/i).waitFor({ timeout: 30000 });
await rec.wait(600);
await rec.spot(row.getByText(/invasor 1\/1/i), "Invasão", "Entrou: o matchmaking achou este mundo na faixa de nível dele e ocupou a vaga de invasor.");
await rec.click(row, { dx: -200 });
await rec.wait(1000);
await rec.spot(page.getByText(/^matinta_/).first().locator("xpath=ancestor::div[contains(@class,'mantine-SimpleGrid-root')][1]"), "Dentro do mundo",
  "O mundo por dentro: o host, as vagas de co-op livres e o invasor que acabou de entrar.", { force: true });
await rec.spot(page.getByText(/Mundo P2P:/).first(), "Confiança",
  "No P2P o jogo do host reporta os eventos. O console deixa isso explícito para quem opera.", { force: true });
await staged;

// --- 8. o fim da partida --------------------------------------------------------------
await rec.caption("Fim da luta", "Agora o invasor derrota o host. O jogo do host reporta a morte…");
await promisify(execFile)(path.join(here, "stage.sh"), ["kill"]);
// o polling tira o mundo das abertas
await page.getByRole("row").filter({ hasText: /steam:/ }).first().waitFor({ state: "detached", timeout: 30000 });
await rec.wait(800);
await rec.say("Fechou", "O mundo saiu da lista de abertas: ele fecha sozinho quando o host morre.");
await rec.click(tab("Analytics"));
await rec.wait(2500);
await rec.spot(page.getByText("Resumo do mundo").locator("xpath=ancestor::div[contains(@class,'mantine-Paper-root') or contains(@class,'mantine-Card-root')][1]"),
  "Analytics", "O analytics do jogo já contou a partida: uma invasão, vencida pelo invasor.", { force: true });
await rec.spot(page.getByText("Saúde das sessões").locator("xpath=ancestor::div[contains(@class,'mantine-Paper-root') or contains(@class,'mantine-Card-root')][1]"),
  "Sessões", "E a saúde das sessões: como cada uma terminou e quanto durou.", { force: true });
await rec.click(tab("Sessions"));
await rec.wait(1500);
await rec.click(page.getByText("Fechadas", { exact: true }).first());
await rec.wait(1200);
const closed = page.getByRole("row").filter({ hasText: /Host morto/ }).first();
await closed.waitFor({ timeout: 20000 });
await rec.spot(closed, "Histórico", "E no histórico de sessões do jogo: fechada como Host morto, com a duração e quem estava dentro.");

await rec.caption(null);
await rec.wait(600);

const frames = await rec.end();
const body = path.join(tmp, "01-body.mp4");
await encodeFrames(frames, body);
await promisify(execFile)(path.join(here, "stage.sh"), ["stop"]);
if (process.env.ONLY_BODY) process.exit(0); // ensaio: só a gravação, sem cartelas
await renderCard({ kicker: "Vídeo 1 · Cadastro de jogo", title: "Do zero ao jogo online",
  sub: "Registrar um jogo, escolher como ele joga online e ver o primeiro mundo abrir e ser invadido." }, 4.5, path.join(tmp, "01-intro.mp4"));
await renderCard({ kicker: "Sem deploy", title: "Nenhuma linha de código<br/>por jogo.",
  sub: "Um formulário configurou matchmaking, sessões, P2P e métricas. O resto do backend leu o modo sozinho." }, 5, path.join(tmp, "01-outro.mp4"));
await concat([path.join(tmp, "01-intro.mp4"), body, path.join(tmp, "01-outro.mp4")], path.join(mediaDir, "video-01-cadastro.mp4"));
console.log("pronto: docs/media/video-01-cadastro.mp4 (jogo " + gameId + ")");
