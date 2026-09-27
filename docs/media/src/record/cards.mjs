// cards.mjs: as cartelas de abertura e fechamento de cada vídeo de feature e o
// passo final (cartela + gravação + cartela). Separado dos roteiros para que o
// hq.mjs remonte as versões em alta qualidade a partir dos quadros já gravados,
// sem gravar de novo.
import path from "node:path";
import { concat, mediaDir, renderCard } from "./lib.mjs";

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

// finish monta o vídeo final de `name` em outDir: cartela, gravação (body) e cartela.
export async function finish(name, body, outDir = mediaDir) {
  const c = CARDS[name];
  const tmp = path.join(mediaDir, "frames");
  const intro = path.join(tmp, `${name.slice(0, 2)}-intro.mp4`);
  const outro = path.join(tmp, `${name.slice(0, 2)}-outro.mp4`);
  await renderCard(c.intro, INTRO_S, intro);
  await renderCard(c.outro, OUTRO_S, outro);
  const out = path.join(outDir, c.file);
  await concat([intro, body, outro], out);
  return out;
}
