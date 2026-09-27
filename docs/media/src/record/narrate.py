#!/usr/bin/env python3
"""narrate.py: o narrador dos vídeos (Kokoro, voz pm_alex, português do Brasil).

Roda como processo de longa duração: o gravador (lib.mjs, com NARRATE=1) manda uma
linha JSON por legenda, {"text": "..."}, e recebe {"file": "<wav>", "dur": <s>}. O
áudio fica em cache por texto, então regravar um vídeo só sintetiza as falas novas.

O texto que vai para a voz passa antes pelo dicionário de pronúncia (PRONUNCIA):
a legenda na tela continua igual; só a fala troca termos em inglês e identificadores
por uma grafia que o fonetizador do português lê direito.

Modelo e voz (uma vez, ~330 MB): Kokoro-82M em ONNX, do Hugging Face
(onnx-community/Kokoro-82M-v1.0-ONNX: onnx/model.onnx e voices/pm_alex.bin), em
KOKORO_DIR (padrão ~/.cache/kokoro). Baixados na primeira execução se faltarem.
Dependências: pip install kokoro-onnx soundfile
"""
import hashlib
import json
import os
import re
import sys
import urllib.request

import numpy as np
import soundfile as sf

VOICE = os.environ.get("NARRATE_VOICE", "pm_alex")
SPEED = float(os.environ.get("NARRATE_SPEED", "1.0"))
KOKORO_DIR = os.environ.get("KOKORO_DIR", os.path.expanduser("~/.cache/kokoro"))
CACHE = os.environ.get("NARRATE_CACHE", os.path.join(os.path.dirname(__file__), "..", "..", "narration"))
HF = "https://huggingface.co/onnx-community/Kokoro-82M-v1.0-ONNX/resolve/main"

# Grafias para a voz. Ordem importa: identificadores compostos antes das palavras.
PRONUNCIA = [
    (r"\bboss_boitata\b", "bóss boitatá"),
    (r"\bchefe_implacavel\b", "chefe implacável"),
    (r"\binvasion_won\b", "invêijon uón"),
    (r"\bGo Multiplayer Control Plane\b", "Gôu Multipléier Contról Plêin"),
    (r"\bcontrol plane\b", "contról plêin"),
    (r"\bem Go\b", "em Gôu"),
    (r"\bnetcode\b", "nét-côud"),
    (r"\bKubernetes\b", "Kubernétis"),
    (r"\bRaspberry Pi\b", "Rásberri Pái"),
    (r"\bbattle royale\b", "bétou roiál"),
    (r"\bAgones\b", "Agônes"),
    (r"\bADR\b", "a dê érre"),
    (r"\brace detector\b", "rêis ditéctor"),
    (r"\bCI\b", "cê í"),
    (r"\bdocker compose\b", "dóquer compôuz"),
    (r"\bGitHub\b", "Guít-rãb"),
    (r"\bspec\b", "spéc"),
    (r"\bID\b", "ái-di"),
    (r"\bSteamID\b", "Stím ái-di"),
    (r"\bSteam\b", "Stím"),
    (r"\bP2P\b", "pê dois pê"),
    (r"\bmatchmaking\b", "métch-mêiking"),
    (r"\bbackend\b", "béc-end"),
    (r"\bdeploy\b", "diplói"),
    (r"\banalytics\b", "anâlitics"),
    (r"\bonline\b", "onláin"),
    (r"\bHosts\b", "Rôusts"),
    (r"\b[Hh]ost\b", "rôust"),
    (r"\bco-op\b", "cô-óp"),
    (r"\bstats\b", "stéts"),
    (r"\bstat\b", "stét"),
    (r"\bkills\b", "kíls"),
    (r"\brankings\b", "rênkings"),
    (r"\bslug\b", "slâg"),
    (r"\bhandle\b", "rêndou"),
    (r"\bnamespace\b", "nêimspêis"),
    (r"\bJSON\b", "djêison"),
    (r"\bsave\b", "sêiv"),
    (r"\bchip\b", "tchíp"),
    (r"\btier\b", "tíer"),
    (r"\bcard\b", "cárd"),
    (r"\bcanvas\b", "cânvas"),
    (r"\bPlayers\b", "Plêiers"),
    (r"\bRooms\b", "Rúms"),
    (r"\bSouls\b", "Sôuls"),
    (r"\bEsc\b", "Ésqui"),
    (r"\bid\b", "ái-di"),
    (r"\b360\b", "trezentos e sessenta"),
    (r"≥", "maior ou igual a"),
    (r"…", "."),
    (r"_", " "),
]


def for_voice(text: str) -> str:
    t = re.sub(r"<[^>]+>", " ", text)  # <br/> das cartelas
    for pat, rep in PRONUNCIA:
        t = re.sub(pat, rep, t)
    return re.sub(r"\s+", " ", t).strip()


def ensure_assets() -> tuple[str, str]:
    os.makedirs(KOKORO_DIR, exist_ok=True)
    model = os.path.join(KOKORO_DIR, "model.onnx")
    voice_bin = os.path.join(KOKORO_DIR, f"{VOICE}.bin")
    for path, url in [(model, f"{HF}/onnx/model.onnx"), (voice_bin, f"{HF}/voices/{VOICE}.bin")]:
        if not os.path.exists(path):
            print(f"narrate: baixando {url}", file=sys.stderr)
            urllib.request.urlretrieve(url, path + ".part")
            os.replace(path + ".part", path)
    voices = os.path.join(KOKORO_DIR, f"voices-{VOICE}.npz")
    if not os.path.exists(voices):
        np.savez(voices, **{VOICE: np.fromfile(voice_bin, dtype=np.float32).reshape(-1, 1, 256)})
    return model, voices


def main() -> None:
    from kokoro_onnx import Kokoro  # importa só aqui: o --text de teste não precisa do modelo

    model, voices = ensure_assets()
    kokoro = Kokoro(model, voices)
    os.makedirs(CACHE, exist_ok=True)
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        text = for_voice(json.loads(line)["text"])
        key = hashlib.sha1(f"{VOICE}|{SPEED}|{text}".encode()).hexdigest()[:16]
        wav = os.path.abspath(os.path.join(CACHE, f"{key}.wav"))
        if not os.path.exists(wav):
            samples, sr = kokoro.create(text, voice=VOICE, speed=SPEED, lang="pt-br")
            sf.write(wav, samples, sr)
        info = sf.info(wav)
        print(json.dumps({"file": wav, "dur": round(info.frames / info.samplerate, 3), "spoken": text}), flush=True)


if __name__ == "__main__":
    if len(sys.argv) > 2 and sys.argv[1] == "--text":
        print(for_voice(sys.argv[2]))  # confere o dicionário sem sintetizar
    else:
        main()
