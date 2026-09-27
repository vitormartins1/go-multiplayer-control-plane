// cards.mjs: as cartelas de abertura e fechamento de cada vídeo de feature e o
// passo final (cartela + gravação + cartela). Separado dos roteiros para que o
// hq.mjs remonte as versões em alta qualidade a partir dos quadros já gravados,
// sem gravar de novo. Com NARRATE=1, as cartelas também são narradas e o passo
// final mixa a narração e escreve a legenda (.srt) para o YouTube.
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { NARRATE, concat, mediaDir, mediaDuration, mixNarration, renderCard, speak, srt, stopNarrator } from "./lib.mjs";

export const CARDS = {
  "01-cadastro": {
    file: "video-01-cadastro.mp4",
    intro: { kicker: "Vídeo 1 · Cadastro de jogo", title: "Do zero ao jogo online",
      sub: "Registrar um jogo, escolher como ele joga online e ver o primeiro mundo abrir e ser invadido." },
    outro: { kicker: "Sem deploy", title: "Nenhuma linha de código<br/>por jogo.",
      sub: "Um formulário configurou matchmaking, sessões, P2P e métricas. O resto do backend leu o modo sozinho." },
  },
  "02-conquistas": {
    file: "video-02-conquistas.mp4",
    intro: { kicker: "Vídeo 2 · Conquistas", title: "O motor de conquistas",
      sub: "Dois tipos de critério, regras com comparadores, globais do arquétipo e regras do jogo, tudo editado no console." },
    outro: { kicker: "Regras são dados", title: "Flexível, sem<br/>código novo.",
      sub: "Métricas acumuladas, eventos com meta, regras compostas e supressão por jogo: cada estúdio monta as conquistas do seu jeito." },
  },
  "03-jogador": {
    file: "video-03-jogador.mp4",
    intro: { kicker: "Vídeo 3 · Jogador", title: "Visão 360 do jogador",
      sub: "Presença, conquistas, histórico, dados, save e a sessão ao vivo de cada jogador, na mesma tela." },
    outro: { kicker: "Uma tela, todos os serviços", title: "Tudo sobre o jogador,<br/>num lugar só.",
      sub: "O console junta o que cada serviço sabe: contas, conquistas, sessões, dados e saves." },
  },
};

const INTRO_S = 4.5;
const OUTRO_S = 5;
const LEAD_S = 0.6; // a fala da cartela começa depois de o título aparecer

const cardText = (c) => `${c.title.replace(/<br\/?>/g, " ")}. ${c.sub}`;

// finish monta o vídeo final de `name` em outDir: cartela, gravação (body) e
// cartela. Com NARRATE=1 sai em docs/media/hq/<vídeo>-narrado.mp4, com a
// narração mixada e a legenda .srt ao lado.
export async function finish(name, body, outDir = NARRATE ? path.join(mediaDir, "hq") : mediaDir) {
  const c = CARDS[name];
  const tmp = path.join(mediaDir, "frames");
  const intro = path.join(tmp, `${name.slice(0, 2)}-intro.mp4`);
  const outro = path.join(tmp, `${name.slice(0, 2)}-outro.mp4`);

  let introS = INTRO_S, outroS = OUTRO_S, introA, outroA;
  if (NARRATE) {
    introA = await speak(cardText(c.intro));
    outroA = await speak(cardText(c.outro));
    introS = Math.max(INTRO_S, LEAD_S + introA.dur + 0.8);
    outroS = Math.max(OUTRO_S, LEAD_S + outroA.dur + 1.2);
  }
  await renderCard(c.intro, introS, intro);
  await renderCard(c.outro, outroS, outro);
  const file = NARRATE ? c.file.replace(/\.mp4$/, "-narrado.mp4") : c.file;
  const out = path.join(outDir, file);
  if (!NARRATE) {
    await concat([intro, body, outro], out);
    return out;
  }

  const silent = path.join(tmp, `${name.slice(0, 2)}-silent.mp4`);
  await concat([intro, body, outro], silent);
  const introDur = await mediaDuration(intro);
  const bodyDur = await mediaDuration(body);
  const rec = JSON.parse(await readFile(path.join(tmp, name, "narration.json"), "utf8"));
  const clips = [
    { at: LEAD_S, file: introA.file, dur: introA.dur, text: cardText(c.intro) },
    ...rec.events.map((e) => ({ at: introDur + shown(rec, e.t), file: e.file, dur: e.dur, text: e.text })),
    { at: introDur + bodyDur + LEAD_S, file: outroA.file, dur: outroA.dur, text: cardText(c.outro) },
  ];
  await mixNarration(silent, clips, out);
  await writeFile(out.replace(/\.mp4$/, ".srt"), srt(clips));
  stopNarrator();
  return out;
}

// shown: instante (s, desde o início do corpo) em que a legenda pedida em t
// aparece na tela: o primeiro quadro depois do pedido, mais metade do fade-in.
function shown(rec, t) {
  const f = rec.frames?.find((ft) => ft >= t) ?? t;
  return f - rec.t0 + 0.15;
}
