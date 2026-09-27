// Vídeo 2 — "O motor de conquistas": os dois critérios (evento e métrica), as
// agregações, as regras globais e por jogo, uma regra nova com comparador, a
// conquista criada a partir do evento órfão e tudo desbloqueando ao vivo.
//
//   node docs/media/src/record/02-conquistas.mjs
//
// Usa o Folclore Souls da demo (modo "default" p2p, stats boss_boitata,
// boss_curupira e kills). Antes de regravar, apague as conquistas mao_pesada e
// implacavel e a regra que emite chefe_implacavel.
import { spawn } from "node:child_process";
import path from "node:path";
import { CONSOLE, concat, encodeFrames, mediaDir, renderCard, startRecording } from "./lib.mjs";

const here = path.dirname(new URL(import.meta.url).pathname);
const GAME_NAME = process.env.GAME_NAME || "Folclore Souls";
const MODE_ID = process.env.MODE_ID || "default";
const tmp = path.join(mediaDir, "frames");

const games = await (await fetch(CONSOLE + "/api/games")).json();
const game = games.find((g) => g.name === GAME_NAME);
if (!game) throw new Error(`jogo "${GAME_NAME}" não encontrado`);
const stageRun = (...args) => new Promise((ok, ko) =>
  spawn(path.join(here, "stage.sh"), args, { env: { ...process.env, INVADE_AFTER: "2" }, stdio: "inherit" })
    .on("close", (c) => (c ? ko(new Error("stage " + args[0])) : ok())));

const rec = await startRecording("02-conquistas");
const { page } = rec;
const side = { left: true, width: 370 }; // modal/drawer ocupando o centro ou a direita
const row = (text) => page.getByRole("row").filter({ hasText: text }).first();
// grafo inteiro legível (admin-ui spec 094): recolhe a lista e o detalhe e
// põe o canvas em tela cheia; o enquadramento automático faz o resto
const node = (type) => page.locator(`.react-flow__node-${type}`);
const collapseIfOpen = async (name) => {
  const b = page.getByRole("button", { name });
  if (await b.isVisible()) await rec.click(b);
};
const showGraph = async () => {
  await collapseIfOpen("Recolher lista");
  await collapseIfOpen("Recolher detalhe");
  await rec.click(page.getByRole("button", { name: "Tela cheia" }));
  await rec.wait(1300);
};
const exitFull = async () => {
  await rec.caption("Esc", "Esc volta o canvas ao tamanho normal.");
  await rec.wait(1600);
  await page.keyboard.press("Escape");
  await rec.wait(1200);
};

await page.goto(`${CONSOLE}/games/${game.id}`);
await page.getByRole("button", { name: "Nova conquista" }).waitFor();
await row("cacador_de_hosts").waitFor();
await rec.begin();

// --- 1. o catálogo: dois critérios -------------------------------------------
await rec.say("Vídeo 2", "O motor de conquistas: como uma conquista é definida, de onde vêm os eventos e como ela desbloqueia.");
await rec.spot(page.locator("table:visible").first(), "Catálogo",
  "Este é o catálogo do Folclore Souls. Cada linha é uma conquista, com o critério e a meta.");
const crit = (text) => row(text).locator("td").nth(4);
await rec.spot(crit("invasor_vitorioso"), "Por evento",
  "Critério por evento: conta quantas vezes um evento aconteceu. Invasor Vitorioso pede 1 vitória numa invasão.");
await rec.spot(row("cacador_de_hosts"), "Mesmo evento",
  "Caçador de Hosts escuta o mesmo evento, com meta 5. Um evento pode alimentar várias conquistas.");
await rec.spot(crit("Chama do Boitatá"), "Por métrica",
  "Critério por métrica: soma uma stat da partida. Aqui, boss_boitata, acumulada entre partidas.");

// --- 2. conquista nova por métrica --------------------------------------------
await rec.click(page.getByRole("button", { name: "Nova conquista" }));
const modal = page.getByRole("dialog");
await rec.wait(500);
await rec.say("Nova", "Vamos criar uma conquista por métrica: 50 abates, somando todas as partidas do jogador.", side);
await rec.type(modal.getByRole("textbox", { name: "Id (slug)" }), "mao_pesada", { delay: 60 });
await rec.type(modal.getByRole("textbox", { name: "Nome", exact: true }), "Mão Pesada", { delay: 60 });
await rec.type(modal.getByRole("textbox", { name: "Descrição" }), "Derrubou 50 inimigos, somando todas as partidas", { delay: 35 });
await rec.spot(modal.locator(".mantine-SegmentedControl-root"), "Critério", "Aqui se escolhe o tipo de critério: por evento ou por métrica.", side);
await rec.click(modal.getByText("Por métrica", { exact: true }));
await rec.wait(600);
await rec.click(modal.getByRole("textbox", { name: "Stat" }));
await rec.wait(800);
await rec.say("Stats", "As stats disponíveis vêm dos modos do jogo. Ao lado de cada uma, o modo que a declara.", side);
await rec.click(page.getByRole("option", { name: /^kills/ }));
await rec.click(modal.getByRole("textbox", { name: "Agregação" }));
await rec.wait(800);
await rec.say("Agregação", "Acumulado soma entre partidas. Por partida vale o melhor resultado numa partida só.", side);
await rec.click(page.getByRole("option", { name: /Acumulado/ }));
const meta = modal.getByRole("textbox", { name: /Meta/ });
await rec.click(meta);
await page.keyboard.press("Control+A");
await page.keyboard.type("50", { delay: 140 });
await rec.type(modal.getByRole("textbox", { name: /Nome na Steam/ }), "ACH_MAO_PESADA", { delay: 55 });
await rec.spot(modal.getByRole("textbox", { name: /Nome na Steam/ }), "Steam",
  "Com o nome da conquista na Steam preenchido, o desbloqueio aparece também no perfil Steam do jogador.", side);
await rec.click(modal.getByRole("button", { name: "Adicionar" }));
await rec.wait(1000);
await rec.spot(row("mao_pesada"), "Catálogo", "Mão Pesada entra no catálogo, com o selo Steam, pronta para contar. Nenhum serviço reiniciou.");

// --- 3. regras: globais do arquétipo, o grafo e a supressão por jogo ---------
await rec.click(page.getByRole("tab", { name: /Event Rules/ }));
await rec.wait(1400);
await rec.say("Regras", "Mas de onde vêm os eventos? Das regras: elas transformam fatos da partida em eventos.");
await rec.spot(page.getByText("INVASION_WON").first().locator("xpath=ancestor::tbody[1]"), "Globais",
  "Estas vêm prontas com o arquétipo do jogo, marcadas como globais. Valem para todo jogo do mesmo tipo.");
await rec.click(page.getByText("INVASION_WON").first());
await rec.wait(1200);
await rec.say("Grafo", "Ao escolher uma regra, o canvas desenha o caminho dela. Primeiro, vamos abrir espaço.");
await rec.spot(page.getByRole("button", { name: "Recolher lista" }), "Espaço",
  "A lista de regras recolhe num botão, e o grafo ganha a largura dela.");
await rec.click(page.getByRole("button", { name: "Recolher lista" }));
await rec.click(page.getByRole("button", { name: "Recolher detalhe" }));
await rec.wait(900);
await rec.spot(page.getByRole("button", { name: "Tela cheia" }), "Tela cheia",
  "E o canvas pode ocupar a tela inteira.");
await rec.click(page.getByRole("button", { name: "Tela cheia" }));
await rec.wait(1300);
await rec.say("Da esquerda para a direita", "Agora o caminho inteiro cabe na tela. Ele se lê da esquerda para a direita.");
await rec.spot(node("source"), "Fatos",
  "Primeiro, os fatos que a partida reporta: o resultado, o papel do jogador e o evento que aconteceu.");
await rec.spot(node("rule"), "Regra",
  "A regra junta condições sobre esses fatos. Esta pede: o host foi morto, com sucesso, por um invasor. Todas precisam bater.");
await rec.spot(node("eventType"), "Evento",
  "Quando todas batem, a regra emite um evento com nome próprio: invasion_won.");
await rec.spot(node("achievement"), "Conquistas",
  "E o evento alimenta as conquistas: Invasor Vitorioso desbloqueia na primeira vez; Caçador de Hosts, na quinta.");
await exitFull();
await rec.click(page.getByRole("button", { name: "Expandir detalhe" }));
await rec.wait(600);
await rec.spot(page.getByRole("button", { name: /Suprimir para este jogo/ }), "Por jogo",
  "Uma regra global não serve para este jogo? Dá para suprimi-la só aqui, sem afetar os outros jogos.");

// --- 4. regra do jogo com comparador + conquista a partir do evento órfão ----
await rec.say("Regra nova", "Agora uma regra do próprio jogo: derrotar um chefe com 10 abates ou mais na mesma luta.");
await rec.click(page.getByRole("button", { name: "Nova regra" }));
await rec.wait(1400); // o enquadramento amplia o rascunho sozinho (spec 094)
const keys = page.getByPlaceholder("chave");
await rec.type(keys.nth(0), "event", { delay: 80 });
await rec.type(page.getByPlaceholder("valor").nth(0), "BOSS_DEFEATED", { delay: 65 });
await rec.caption("Condição 1", "Primeira condição: o evento da partida é a derrota de um chefe.");
await rec.wait(1800);
await rec.click(page.getByRole("button", { name: "Adicionar condição" }));
await rec.wait(500);
await rec.type(keys.nth(1), "stat.kills", { delay: 80 });
await rec.click(page.getByRole("button", { name: "Operador" }).nth(1));
await rec.wait(900);
await rec.say("Comparador", "Segunda condição, com comparador: os abates do jogador naquela luta, maior ou igual a 10.");
await rec.click(page.getByRole("button", { name: "≥", exact: true }));
await rec.type(page.getByPlaceholder("nº").first(), "10", { delay: 140 });
await rec.type(page.getByPlaceholder("nome_do_type"), "chefe_implacavel", { delay: 65 });
await rec.say("Evento novo", "Quando as duas condições batem, a regra emite um evento novo, com o nome que o estúdio escolher.");
await rec.click(page.getByRole("button", { name: "Salvar", exact: true }));
await rec.wait(2000);
await rec.click(page.getByRole("button", { name: "Mostrar lista de regras" }));
await rec.wait(900);
await rec.spot(page.getByText("chefe_implacavel").first().locator("xpath=ancestor::tr[1]"), "Regra do jogo",
  "Ela entra na lista ao lado das globais, marcada como regra do jogo.");
await rec.click(page.getByText("chefe_implacavel").first()); // o item novo na lista
await rec.wait(1200);
await rec.click(page.getByRole("button", { name: "Recolher lista" }));
await rec.wait(1200);
await rec.click(node("eventType"));
await rec.wait(1000);
await rec.spot(page.getByRole("button", { name: /Criar achievement que escuta este type/ }), "Órfão",
  "O evento ainda não alimenta nenhuma conquista. O canvas avisa e oferece criar a conquista ali mesmo.");
await rec.click(page.getByRole("button", { name: /Criar achievement que escuta este type/ }));
const drawer = page.locator(".mantine-Drawer-content");
await rec.wait(700);
await rec.say("Conquista", "Critério por evento, meta 1: desbloqueia na primeira vez que acontecer.", side);
await rec.type(drawer.getByRole("textbox", { name: "ID" }), "implacavel", { delay: 55 });
await rec.type(drawer.getByRole("textbox", { name: "Nome" }), "Implacável", { delay: 55 });
await rec.type(drawer.getByRole("textbox", { name: "Descrição" }), "Derrotou um chefe com 10 abates ou mais", { delay: 35 });
await rec.wait(600);
await rec.click(drawer.getByRole("button", { name: "Criar" }));
await rec.wait(1600); // a regra continua focada; o grafo ganha a conquista nova
await showGraph();
await rec.spot(node("source"), "Fatos", "Os fatos desta regra: o evento da partida e a stat kills, com os abates da luta.");
await rec.spot(node("rule"), "Comparador", "A regra do jogo: chefe derrotado e kills maior ou igual a 10.");
await rec.spot(node("achievement"), "Fechado",
  "O caminho fecha: o evento chefe_implacavel alimenta a conquista Implacável, criada agora há pouco.");
await exitFull();

// --- 5. ao vivo ---------------------------------------------------------------
const staged = stageRun("world", game.id, MODE_ID).then(() => stageRun("boss", "14"));
await rec.click(page.getByRole("tab", { name: /Achievements/ }));
await rec.caption("Ao vivo", "Agora, num mundo do Folclore Souls, o host derrota o Boitatá com 14 abates.");
await staged;
await rec.wait(3200); // fan-out: SNS → SQS → consumer
await rec.click(row("mao_pesada").getByRole("button", { name: "Ver progresso" }));
await rec.wait(900);
await rec.spot(page.locator(".mantine-Drawer-content table, .mantine-Modal-content table").first(), "Métrica",
  "Mão Pesada avançou 14 de 50. Ela continua somando nas próximas partidas desse jogador.", side);
await rec.click(page.getByRole("button", { name: "Fechar", exact: true }));
await rec.wait(600);
await rec.click(row("implacavel").getByRole("button", { name: "Ver progresso" }));
await rec.wait(900);
await rec.spot(page.locator(".mantine-Drawer-content table, .mantine-Modal-content table").first(), "Regra",
  "Implacável desbloqueou pela regra nova, na hora. Nenhum deploy, nenhuma linha de código.", side);
await rec.caption(null);
await rec.wait(500);
await stageRun("stop");

const frames = await rec.end();
const body = path.join(tmp, "02-body.mp4");
await encodeFrames(frames, body);
if (process.env.ONLY_BODY) process.exit(0); // ensaio: só a gravação, sem cartelas
await renderCard({ kicker: "Vídeo 2 · Conquistas", title: "O motor de conquistas",
  sub: "Dois tipos de critério, regras com comparadores, globais do arquétipo e regras do jogo, tudo editado no console." }, 4.5, path.join(tmp, "02-intro.mp4"));
await renderCard({ kicker: "Regras são dados", title: "Flexível, sem<br/>código novo.",
  sub: "Métricas acumuladas, eventos com meta, regras compostas e supressão por jogo: cada estúdio monta as conquistas do seu jeito." }, 5, path.join(tmp, "02-outro.mp4"));
await concat([path.join(tmp, "02-intro.mp4"), body, path.join(tmp, "02-outro.mp4")], path.join(mediaDir, "video-02-conquistas.mp4"));
console.log("pronto: docs/media/video-02-conquistas.mp4");
