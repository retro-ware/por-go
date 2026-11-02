"""Renders the voice-over (engine/voice.go) into GAMEDIR/SPEECH/<key>.ogg (Ogg Vorbis).

    ~/.mlx-audio-env/bin/python voice/render.py GAMEDIR

Kokoro-82M (mlx-community/Kokoro-82M-bf16), voice am_michael. Keys: the
script texts of engine/journal/LINES.TXT by FNV-64a hash (as voiceKey), the journal
texts by title ("Journal_Entry_26"), Rolf's history (HISTORY.TXT) by chapter
title and paragraph hash. Files already rendered are kept: delete
one to render it again. Needs oggenc (brew install vorbis-tools).
"""
import re, subprocess, sys
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
    for l in (ROOT / 'engine/journal/LINES.TXT').read_text().splitlines():
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


def history():
    """engine/journal/HISTORY.TXT (Rolf's history of Phlan): each chapter's
    title by name, as the journal's, each paragraph by its hash."""
    for l in (ROOT / 'engine/journal/HISTORY.TXT').read_text().splitlines():
        l = l.strip()
        if not l or l.startswith('#'):
            continue
        if l.startswith('[') and l.endswith(']'):
            yield l[1:-1].replace(' ', '_'), l[1:-1].replace('&', 'and') + '.'
        else:
            yield key(l), l


def main():
    out = Path(sys.argv[1]) / 'SPEECH'
    out.mkdir(parents=True, exist_ok=True)
    todo = [(k, t) for k, t in [*texts(), *history()] if not (out / (k + '.ogg')).exists()]
    model = load_model('mlx-community/Kokoro-82M-bf16')
    for i, (k, t) in enumerate(todo, 1):
        audio = np.concatenate([np.array(r.audio) for r in model.generate(t, voice='am_michael', lang_code='a', speed=1.0)])
        pcm = (np.clip(audio, -1, 1) * 32767).astype(np.int16).tobytes()
        subprocess.run(['oggenc', '-Q', '-r', '-R', str(model.sample_rate), '-C', '1', '-B', '16',
                        '--resample', '44100', '-q', '4', '-o', str(out / (k + '.ogg')), '-'], input=pcm, check=True)
        print(f'{i}/{len(todo)} {k} {t[:60]!r}', flush=True)


if __name__ == '__main__':
    main()
