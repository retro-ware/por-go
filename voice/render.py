"""Renders the voice-over (engine/voice.go) into GAMEDIR/SPEECH/<key>.mp3.

    ~/.mlx-audio-env/bin/python voice/render.py GAMEDIR

Kokoro-82M (mlx-community/Kokoro-82M-bf16), voice am_michael. Keys: the
script texts of voice/lines.txt by FNV-64a hash (as voiceKey), the journal
texts by title ("Journal_Entry_26"). Files already rendered are kept: delete
one to render it again. Needs ffmpeg with libmp3lame.
"""
import re, subprocess, sys, tempfile, wave
from pathlib import Path

import numpy as np
from mlx_audio.tts.utils import load_model

ROOT = Path(__file__).resolve().parent.parent


def key(s):
    h = 0xcbf29ce484222325
    for b in s.strip().encode():
        h = ((h ^ b) * 0x100000001b3) & 0xFFFFFFFFFFFFFFFF
    return format(h, 'x')


def spoken(s):
    """The game's capitals read as words: sentence case, 'I' kept."""
    if s.upper() != s:
        return s
    s = s.lower()
    s = re.sub(r"(^|[.!?]\s+|(?:^|\s)['\"])([a-z])", lambda m: m.group(1) + m.group(2).upper(), s)
    return re.sub(r"\bi\b", "I", s)


def texts():
    for l in (ROOT / 'voice/lines.txt').read_text().splitlines():
        if l.strip() and not l.startswith('#'):
            yield key(l), spoken(l)
    title, paras = None, []
    for l in (ROOT / 'engine/journal/JOURNAL.TXT').read_text().replace('\r', '').splitlines() + ['[]']:
        l = l.strip()
        if l.startswith('#') or (l.startswith('<') and l.endswith('>')):
            continue
        if l.startswith('[') and l.endswith(']'):
            if title:
                yield title.replace(' ', '_'), title + '.\n' + '\n'.join(p for p in paras if p)
            title, paras = l[1:-1], []
        elif title:
            paras.append(l)


def main():
    out = Path(sys.argv[1]) / 'SPEECH'
    out.mkdir(parents=True, exist_ok=True)
    todo = [(k, t) for k, t in texts() if not (out / (k + '.mp3')).exists()]
    model = load_model('mlx-community/Kokoro-82M-bf16')
    for i, (k, t) in enumerate(todo, 1):
        audio = np.concatenate([np.array(r.audio) for r in model.generate(t, voice='am_michael', lang_code='a', speed=1.0)])
        with tempfile.NamedTemporaryFile(suffix='.wav') as w:
            with wave.open(w.name, 'wb') as f:
                f.setnchannels(1)
                f.setsampwidth(2)
                f.setframerate(model.sample_rate)
                f.writeframes((np.clip(audio, -1, 1) * 32767).astype(np.int16).tobytes())
            subprocess.run(['ffmpeg', '-loglevel', 'error', '-y', '-i', w.name, '-ar', '44100', '-ac', '1',
                            '-b:a', '48k', str(out / (k + '.mp3'))], check=True)
        print(f'{i}/{len(todo)} {k} {t[:60]!r}', flush=True)


if __name__ == '__main__':
    main()
