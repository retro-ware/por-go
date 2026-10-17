#!/usr/bin/env python3
"""Generate floor, wall or ceiling textures with the OpenAI or Gemini image API.
  python3 tools/new-texture.py                      interactive
  python3 tools/new-texture.py KIND NAME "DESC"...  one or more textures, no questions
Uses $OPENAI_API_KEY or ~/.config/blobber/openai_key, else the Gemini key new-monster.py keeps. Writes
assets/walls/<kind>_<name>.png (256x256; the model is asked for seamless output, nothing is post-processed). A wall also gets a map character in
assets/walls/walls.txt, so it shows up in the editor's material palette; floors and ceilings show up
in the editor's floor/ceiling row (a map picks one with a "floor NAME" / "ceiling NAME" line).
OPENAI_IMAGE_MODEL overrides the model."""
import base64, io, json, os, re, string, sys, urllib.error, urllib.request
from pathlib import Path
from PIL import Image

MODEL = os.environ.get("OPENAI_IMAGE_MODEL", "gpt-image-2")
KEYFILE = Path.home() / ".config/blobber/openai_key"
GEMINI_KEYFILE = Path.home() / ".config/blobber/gemini_key"
ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "assets/walls"
TABLE = OUT / "walls.txt"
RAW = Path.home() / "Pictures/blobber-textures"  # untouched originals and tiled previews
SIZE = 256
RESERVED = set(".#DLBMWI")  # floor, stone, door, locked door, generated materials

PROMPT = ("Give me a slick square, repeatable dungeon {kind} texture: {desc}. Make it bearable to look "
          "at even with infinite repetition. {view} Seamless tiling on all four edges, even flat lighting, "
          "no vignette, no single light source, no objects, nothing that stands out.")
VIEW = {"floor": "Seen from directly above.",
        "ceiling": "Seen from directly below.",
        "wall": "A flat wall seen straight on, no perspective, filling the whole square."}


def api_key():
    if os.environ.get("OPENAI_API_KEY"):
        return os.environ["OPENAI_API_KEY"]
    if KEYFILE.exists():
        return KEYFILE.read_text().strip()
    if GEMINI_KEYFILE.exists():
        return "gemini:" + GEMINI_KEYFILE.read_text().strip()
    key = input("OpenAI API key: ").strip()
    KEYFILE.parent.mkdir(parents=True, exist_ok=True)
    KEYFILE.write_text(key)
    KEYFILE.chmod(0o600)
    return key


SKY = ("A wide painted fantasy sky for a Might and Magic style RPG, seen from the ground looking at the "
       "horizon: {desc}. Only sky and clouds, no land, no sun disc, no horizon line, no text. Even lighting "
       "across the whole width. Painterly, saturated, crisp.")


def generate(key, prompt, size="1024x1024"):
    if key.startswith("gemini:"):
        import importlib.util
        spec = importlib.util.spec_from_file_location("new_monster", Path(__file__).with_name("new-monster.py"))
        new_monster = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(new_monster)
        new_monster.CONFIG["imageConfig"]["aspectRatio"] = "1:1"
        return new_monster.generate(key[7:], prompt)[0]
    body = {"model": MODEL, "prompt": prompt, "size": size, "n": 1}
    req = urllib.request.Request("https://api.openai.com/v1/images/generations", json.dumps(body).encode(),
                                 {"Content-Type": "application/json", "Authorization": f"Bearer {key}"})
    try:
        with urllib.request.urlopen(req, timeout=300) as r:
            return base64.b64decode(json.load(r)["data"][0]["b64_json"])
    except urllib.error.HTTPError as e:
        sys.exit(f"OpenAI error {e.code}: {e.read().decode()}")


def free_char():
    used = RESERVED | {l.split()[0] for l in TABLE.read_text().splitlines() if l.strip() and not l.startswith("#")}
    return next(c for c in string.ascii_uppercase + string.digits if c not in used)


def create(key, kind=None, name=None, desc=None):
    batch = kind is not None
    if not batch:
        kind = {"f": "floor", "w": "wall", "c": "ceiling"}.get(input("Texture: [f]loor, [w]all or [c]eiling? ").strip().lower()[:1])
        if not kind:
            return
        desc = input(f"Describe the {kind} (e.g. 'dirty and dusty, hard to tell stone covered with dirt from dirt covered with stone'): ").strip()
        default = re.sub(r"[^a-z0-9]+", "_", desc.lower()).strip("_")[:24].strip("_")
        name = input(f"Name [{default}]: ").strip() or default
    dest = OUT / f"{kind}_{name}.png"
    if dest.exists() and not batch and input(f"{dest} exists, overwrite? [y/N] ").lower() != "y":
        return
    raw = RAW / f"{kind}_{name}"
    raw.mkdir(parents=True, exist_ok=True)
    while True:
        print(f"Generating {kind} texture…")
        if kind == "ceiling" and name.startswith("sky"):  # a sky is a panorama the game mirrors around itself, kept full size
            png = generate(key, SKY.format(desc=desc), "1536x1024")
            (raw / "original.png").write_bytes(png)
            tex = Image.open(io.BytesIO(png)).convert("RGB")
        else:
            png = generate(key, PROMPT.format(kind=kind, desc=desc, view=VIEW[kind]))
            (raw / "original.png").write_bytes(png)
            tex = Image.open(io.BytesIO(png)).convert("RGB").resize((SIZE, SIZE), Image.LANCZOS)
        tex.save(dest)
        preview = Image.new("RGB", (tex.width * 3, tex.height * 3))
        for i in range(9):
            preview.paste(tex, (i % 3 * tex.width, i // 3 * tex.height))
        preview.save(raw / "tiled.png")
        print(f"Saved {dest}; 3x3 tiling preview: {raw / 'tiled.png'}")
        if batch or input("Regenerate? [y/N] ").strip().lower() != "y":
            break
    if kind == "wall":
        rows = {l.split()[1]: l.split()[0] for l in TABLE.read_text().splitlines() if l.strip() and not l.startswith("#")}
        ch = rows.get(dest.stem)
        if not ch:
            ch = free_char()
            with TABLE.open("a") as f:
                f.write(f"{ch} {dest.stem}\n")
        print(f"Wall material '{ch}': paint it in the editor, or use {ch} in a .map grid.")
    else:
        print(f"Pick it in the editor's floor/ceiling row, or add \"{kind} {name}\" to a .map file.")


def main():
    key = api_key()
    if len(sys.argv) > 1:
        for i in range(1, len(sys.argv), 3):
            create(key, *sys.argv[i:i + 3])
        return
    while True:
        create(key)
        if input("Create another texture? [y/N] ").strip().lower() != "y":
            break


if __name__ == "__main__":
    main()
