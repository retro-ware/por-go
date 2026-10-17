#!/usr/bin/env python3
"""Generate a new monster (idle, attack, hurt poses) with Gemini image generation.
  python3 tools/new-monster.py
Asks for the Gemini API key once (kept in ~/.config/blobber/gemini_key), then for the
monster, and loops until you say no more. Writes cmd/blobber/assets/monsters/<name>/{idle,attack,hurt}.png:
the chroma-key background (magenta, or green for pink/purple monsters) made transparent, cropped (full resolution: the game resamples to one pixel density), and adds a maps/monsters.txt row. GEMINI_MODEL overrides the model."""
import base64, io, json, os, re, sys, urllib.error, urllib.request
from pathlib import Path
import numpy as np
from PIL import Image, ImageDraw, ImageFilter

MODEL = os.environ.get("GEMINI_MODEL", "gemini-3.1-flash-image")  # Nano Banana 2; gemini-3-pro-image is Pro
CONFIG = {"responseModalities": ["TEXT", "IMAGE"], "temperature": 0.5,  # lower = poses drift less
          "imageConfig": {"aspectRatio": "3:4"}}  # tall canvas, so legs are not cut off
KEYFILE = Path.home() / ".config/blobber/gemini_key"
OUT = Path(__file__).resolve().parent.parent / "cmd/blobber/assets/monsters"
TABLE = Path(__file__).resolve().parent.parent / "maps/monsters.txt"
RAW = Path.home() / "Pictures/blobber-monsters"  # untouched originals; assets/monsters is embedded whole

KEYS = {"magenta": ("#FF00FF", (255, 0, 255)), "green": ("#00FF00", (0, 255, 0))}
BG = "Solid flat pure {name} ({hex}) background for chroma keying, no shadow or floor, no {name} on the monster."
BASE = ("Comic style, only the character: I need a full frontal picture of {desc} in the style of a "
        "dungeon monster, as suitable for a Wizardry style game, that especially means the monster should "
        "look directly at the camera/audience. Whole body in frame. " + BG)
# Sent as the next chat turn with the base image attached, like a follow-up on the Gemini website.
POSE = ("Give me that same monster in a \"{pose}\" pose, still facing me. Redraw the whole monster in "
        "the new pose: it appears exactly once, nothing of the first picture stays in place or shows "
        "through, no second head or body painted over it. " + BG)
# The attack frame is the hit itself (and runs hotter, 0.75, so the body moves, not just the weapon): without this, models draw the wind-up or strike the ground.
ATTACK = (", caught at the exact moment the attack hits me, the viewer: the body lunges forward into the "
          "strike, its whole body moves, the weapon, claws, bite or breath comes straight at the camera, "
          "foreshortened, with an impact flash where it hits")


def api_key():
    if KEYFILE.exists():
        return KEYFILE.read_text().strip()
    key = input("Gemini API key: ").strip()
    KEYFILE.parent.mkdir(parents=True, exist_ok=True)
    KEYFILE.write_text(key)
    KEYFILE.chmod(0o600)
    return key


def generate(key, prompt, history=(), image=None, temperature=None):
    """Returns (PNG bytes, the model's reply turn) for prompt, continuing the chat history (a list
    of contents turns), with image (PNG/JPEG bytes) attached. The reply turn goes back verbatim,
    thought signatures included."""
    parts = [{"text": prompt}]
    if image:
        mime = "image/png" if image[:4] == b"\x89PNG" else "image/jpeg"
        parts.insert(0, {"inline_data": {"mime_type": mime, "data": base64.b64encode(image).decode()}})
    contents = [*history, {"role": "user", "parts": parts}]
    body = {"contents": contents, "generationConfig": {**CONFIG, **({"temperature": temperature} if temperature else {})}}
    req = urllib.request.Request(
        f"https://generativelanguage.googleapis.com/v1beta/models/{MODEL}:generateContent",
        json.dumps(body).encode(), {"Content-Type": "application/json", "x-goog-api-key": key})
    for attempt in range(3):
        try:
            with urllib.request.urlopen(req, timeout=300) as r:
                res = json.load(r)
        except urllib.error.HTTPError as e:
            sys.exit(f"Gemini error {e.code}: {e.read().decode()}")
        reply = res.get("candidates", [{}])[0].get("content", {})
        for p in reply.get("parts", []):
            data = p.get("inlineData") or p.get("inline_data")
            if data:
                return base64.b64decode(data["data"]), {"role": "model", "parts": reply["parts"]}
        print(f"  no image returned, retrying ({attempt + 1}/3)…")
    sys.exit(f"Gemini returned no image: {json.dumps(res)[:500]}")


def keyness(a, key):
    """How strongly each pixel leans toward the key hue: for magenta min(R, B) - G, for green
    G - max(R, B). Brightness and gradients in the background barely change it."""
    on, off = a[..., [i for i in range(3) if key[i]]], a[..., [i for i in range(3) if not key[i]]]
    return on.min(axis=2) - off.max(axis=2)


def cutout(png, key):
    """Background -> transparent, cropped. When the model used the key hue (any shade
    of it), every pixel is keyed by how much of that hue it has, enclosed pockets and glows over
    the background included. Otherwise (e.g. it painted white) the border colour is keyed, but
    only where it touches the border. Edges get partial alpha with the background subtracted."""
    a = np.asarray(Image.open(io.BytesIO(png)).convert("RGB"), dtype=np.float32)
    h, w, _ = a.shape
    border = np.concatenate([a[0], a[-1], a[:, 0], a[:, -1]])
    k = np.median(border, axis=0)
    kbg = keyness(k[None, None], key)[0, 0]
    if kbg > 50:
        t = keyness(a, key) / kbg  # 1 = pure background, 0 = no key hue at all
        alpha = np.clip((0.8 - t) / 0.5, 0, 1)  # t <= 0.3: solid, t >= 0.8: gone
    else:
        print(f"  note: model ignored the key colour, keying the border colour {k.astype(int)} instead")
        alpha = np.clip((np.linalg.norm(a - k, axis=2) - 50) / 70, 0, 1)
        mask = Image.fromarray(np.where(alpha < 1, 255, 0).astype(np.uint8), "L").copy()  # copy: fromarray images ignore floodfill
        for x, y in [(x, y) for x in range(0, w, 4) for y in (0, h - 1)] + [(x, y) for y in range(0, h, 4) for x in (0, w - 1)]:
            if mask.getpixel((x, y)) == 255:
                ImageDraw.floodfill(mask, (x, y), 128)
        outside = np.asarray(mask.point(lambda v: 255 if v == 128 else 0).filter(ImageFilter.MaxFilter(5))) > 0
        alpha = np.where(outside, alpha, 1)
    alpha = alpha[..., None]
    rgb = np.clip((a - (1 - alpha) * k) / np.maximum(alpha, 1e-3), 0, 255)
    # Despill: outline pixels near the edge are dark mixes with the key, solid but tinted; strip
    # the key hue from everything within 4 px of transparency.
    near = np.asarray(Image.fromarray(np.where(alpha[..., 0] < 1, 255, 0).astype(np.uint8)).filter(ImageFilter.MaxFilter(9))) > 0
    on, off = [i for i in range(3) if key[i]], [i for i in range(3) if not key[i]]
    spill = np.clip(rgb[..., on].min(axis=2) - rgb[..., off].max(axis=2), 0, None) * near
    rgb[..., on] -= spill[..., None]
    return Image.fromarray(np.dstack([rgb, alpha * 255]).astype(np.uint8), "RGBA")


def frame(imgs):
    """Crops all poses with one shared box, so the monster keeps its size and place between poses.
    No downscaling: the game resamples every monster to its own pixel density."""
    l, t, r, b = zip(*(i.getbbox() for i in imgs))
    box = (min(l), min(t), max(r), max(b))
    return [i.crop(box) for i in imgs]


def create(key):
    desc = input("Describe the monster (e.g. 'a menacing red dragon'): ").strip()
    attack = input("Attack pose (e.g. 'breathing an intense stream of fire at me') [attacking me]: ").strip() or "attacking me head-on"
    default = re.sub(r"[^a-z0-9]+", "_", re.sub(r"^(an?|the)\s+", "", desc.lower())).strip("_")
    name = input(f"Folder name [{default}]: ").strip() or default
    d = OUT / name
    if d.exists() and input(f"{d} exists, overwrite? [y/N] ").lower() != "y":
        return
    d.mkdir(parents=True, exist_ok=True)

    chroma = "green" if re.search(r"pink|magenta|purple|violet|fuchsia|lilac", desc, re.I) else "magenta"
    bg = {"name": chroma, "hex": KEYS[chroma][0]}
    print(f"Generating idle (base) image on {chroma}…")
    first = BASE.format(desc=desc, **bg)
    base, reply = generate(key, first)
    history = [{"role": "user", "parts": [{"text": first}]}, reply]
    raw = RAW / name
    raw.mkdir(parents=True, exist_ok=True)
    poses = {"idle": base}
    prompts = {"attack": (attack + ATTACK, 0.75), "hurt": ("getting hurt by hard attack", 0.6)}
    todo = list(prompts)
    while todo:
        for pose in todo:
            print(f"Generating {pose}…")
            text, temp = prompts[pose]
            poses[pose], _ = generate(key, POSE.format(pose=text, **bg), history, base, temp)
        for pose, png in poses.items():
            (raw / f"{pose}.png").write_bytes(png)
        for pose, img in zip(poses, frame([cutout(png, KEYS[chroma][1]) for png in poses.values()])):
            img.save(d / f"{pose}.png")
        print(f"Check the poses in {d} (or the originals in {raw}).")
        again = input("Regenerate a pose? [a]ttack, [h]urt, [b]oth, Enter to keep: ").strip().lower()
        todo = {"a": ["attack"], "h": ["hurt"], "b": ["attack", "hurt"]}.get(again[:1], [])
    rows = [l.split()[0] for l in TABLE.read_text().splitlines() if l.strip() and not l.startswith("#")]
    if name not in rows:
        stats = input("Stats: hp ac dmg exp spd [size] [20 5 6 50 10 0.6]: ").split() or "20 5 6 50 10 0.6".split()
        with TABLE.open("a") as f:
            f.write(f"{name} {name} {' '.join(stats)}\n")
        print(f"Added {name} to {TABLE}")
    print(f"Done: {d}  (originals in {raw})")


def main():
    key = api_key()
    while True:
        create(key)
        if input("Create another monster? [y/N] ").strip().lower() != "y":
            break


if __name__ == "__main__":
    main()
