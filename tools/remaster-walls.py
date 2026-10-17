#!/usr/bin/env python3
"""Proof of concept: remastered 3D-view walls from the viewdump PNGs, with
both Gemini and OpenAI image models, each given the original as reference.
  python3 tools/remaster-walls.py [OUT]     (default: remaster/)
Needs the Go toolchain (it runs cmd/viewdump -transparent) and the API keys
BlobberRPG keeps in ~/.config/blobber/ (gemini_key, openai_key). The empty
parts of a wall (sky, ground, gaps) go to the models as a flat key colour
(magenta, green or blue, whichever the wall itself uses least) and come
back as transparency, so a texture holds only what is drawn on the wall.
Writes per wall the reference, NAME_gemini.png, NAME_openai.png (RGBA),
the models' untouched output under raw/ and the prompt used."""
import base64, importlib.util, io, json, subprocess, sys, tempfile, urllib.error, urllib.request, uuid
import numpy as np
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
BLOBBER = Path.home() / "Projects/BlobberRPG/tools/new-monster.py"
KEYS = Path.home() / ".config/blobber"
OPENAI_MODEL = "gpt-image-2"

CONTEXT = (
    "This is a wall graphic from the first-person 3D view of 'Pool of Radiance' (SSI, 1988), the first "
    "AD&D Gold Box computer RPG, set in the ruined city of Phlan on the Moonsea in the Forgotten Realms. "
    "The attached reference is the original 16-colour EGA art (56x64 pixels, upscaled): the wall face the "
    "player sees when standing directly in front of it, one map square wide. Recreate it as a modern, "
    "high-resolution remaster for a remake of that 3D view: same subject, same layout and composition, "
    "same placement and proportions of every element, same colour scheme, but painted in rich detail "
    "with believable materials, subtle weathering and soft even lighting, in a painterly high-fantasy "
    "style like late-1980s TSR book covers rendered with modern quality. Straight-on frontal "
    "orthographic view, no perspective, no camera tilt, no vignette, no text, no frame, no characters. "
    "The reference is stretched to a square; the wall fills the whole image edge to edge. This is a "
    "texture for a game engine that draws sky and ground separately: every area that is flat {kname} "
    "({khex}) in the reference is empty, nothing is drawn there. Keep exactly those areas flat pure "
    "{khex} with crisp edges, no gradient, no shadow on it; do not paint sky, clouds, ground, earth, "
    "horizon, or anything behind or in front of the wall, and use no "
    "{kname} anywhere on the wall itself. What the reference shows: ")

# Chroma keys: name, hex, which channels are "on" (new-monster.keyness).
KEYS_RGB = {"magenta": ("#FF00FF", (255, 0, 255)), "green": ("#00FF00", (0, 255, 0)), "blue": ("#0000FF", (0, 0, 255))}

WALLS = {
    "brick_building_stairs": ("WALLDEF3-000-3-wall2",
        "the front of a red brick town building in Phlan: an even wall of red bricks across the whole "
        "width, thin dark red vertical edges at the far left and right, in the lower middle a wide "
        "window of blue glass panes in a stone frame (two rows of three panes), and below it broad grey "
        "stone steps leading up to the building across the bottom."),
    "columns_over_water": ("WALLDEF3-000-3-wall4",
        "a colonnade by the water: four round red-and-orange stone pillars standing in a row in front "
        "of a stretch of rippling blue water with small white wave crests, the pillars rising from the "
        "water's edge; above the water and between the pillar tops nothing is drawn."),
    "temple_gate": ("WALLDEF1-005-1-wall1",
        "the facade of a temple: a tall pointed gothic gable of dark brown and maroon checkered "
        "brickwork with a round window near its peak, flanked left and right by slender yellow-green "
        "stone towers topped with small dark spires; at the bottom centre a pointed-arch doorway with a "
        "purple stone frame and a dark wooden door of vertical planks."),
    "fountain": ("WALLDEF1-001-1-wall2",
        "a town square fountain against a plain grey stone wall: a wide oval basin of deep blue water "
        "rimmed with grey stone, a tall white and pale cyan water jet rising from its centre, thin teal "
        "edges at the left and right of the wall; the basin's lower rim sits at the bottom edge."),
    "ivy_wall_door": ("WALLDEF1-003-1-wall5",
        "a grey stone wall overgrown with climbing ivy and green vines that grow thickest towards the "
        "bottom and thin out towards the top, with a sturdy arched wooden door of vertical brown planks "
        "with a metal handle at the bottom centre."),
    "bamboo_wall": ("WALLDEF7-007-1-wall1",
        "a wall of tall yellow bamboo canes standing side by side, lashed together with red-brown cord "
        "bands at regular heights, topped with a thick ragged layer of brown, gold and black dry thatch "
        "or brush along the top, as in a jungle or wilderness palisade."),
}


def blobber():
    spec = importlib.util.spec_from_file_location("new_monster", BLOBBER)
    nm = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(nm)
    nm.CONFIG["imageConfig"]["aspectRatio"] = "1:1"
    return nm


NM = blobber()


def pick_key(im):
    """The key colour the wall's own pixels lean towards least."""
    a = np.asarray(im, dtype=np.float32)
    c = a[..., :3][a[..., 3] > 0]
    return min(KEYS_RGB, key=lambda k: NM.keyness(c[None], KEYS_RGB[k][1])[0].max(initial=0))


def reference(dump, name):
    """The wall (transparent dump) on its key colour, stretched to 1024x1024 nearest-neighbour."""
    im = Image.open(dump / f"{name}.png").convert("RGBA")
    k = pick_key(im)
    bg = Image.new("RGBA", im.size, KEYS_RGB[k][1] + (255,))
    bg.alpha_composite(im)
    buf = io.BytesIO()
    bg.convert("RGB").resize((1024, 1024), Image.NEAREST).save(buf, "PNG")
    return buf.getvalue(), k


def key_out(png, k):
    """Key colour -> transparent, as new-monster.cutout does when the model kept the key hue,
    but always keyed against the pure key (a wall may touch no border with it)."""
    key = KEYS_RGB[k][1]
    a = np.asarray(Image.open(io.BytesIO(png)).convert("RGB"), dtype=np.float32)
    kv = np.array(key, dtype=np.float32)
    t = NM.keyness(a, key) / 255
    alpha = np.clip((0.8 - t) / 0.5, 0, 1)[..., None]
    rgb = np.clip((a - (1 - alpha) * kv) / np.maximum(alpha, 1e-3), 0, 255)
    return Image.fromarray(np.dstack([rgb, alpha * 255]).astype(np.uint8), "RGBA")


def gemini(prompt, ref):
    return NM.generate((KEYS / "gemini_key").read_text().strip(), prompt, image=ref)[0]


def openai(prompt, ref):
    key = (KEYS / "openai_key").read_text().strip()
    b = uuid.uuid4().hex
    fields = {"model": OPENAI_MODEL, "prompt": prompt, "size": "1024x1024", "n": "1"}
    body = b"".join(f'--{b}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n'.encode()
                    for k, v in fields.items())
    body += (f'--{b}\r\nContent-Disposition: form-data; name="image[]"; filename="ref.png"\r\n'
             f"Content-Type: image/png\r\n\r\n").encode() + ref + f"\r\n--{b}--\r\n".encode()
    req = urllib.request.Request("https://api.openai.com/v1/images/edits", body,
                                 {"Content-Type": f"multipart/form-data; boundary={b}",
                                  "Authorization": f"Bearer {key}"})
    try:
        with urllib.request.urlopen(req, timeout=600) as r:
            return base64.b64decode(json.load(r)["data"][0]["b64_json"])
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"OpenAI error {e.code}: {e.read().decode()[:500]}")


def job(out, dump, title, src, desc, engine):
    ref, k = reference(dump, src)
    prompt = CONTEXT.format(kname=k, khex=KEYS_RGB[k][0]) + desc
    (out / f"{title}_reference.png").write_bytes(ref)
    (out / f"{title}_prompt.txt").write_text(prompt + "\n")
    try:
        png = (gemini if engine == "gemini" else openai)(prompt, ref)
    except BaseException as e:  # new-monster.generate calls sys.exit on errors
        return f"{title} {engine}: FAILED {e}"
    (out / "raw").mkdir(exist_ok=True)
    (out / "raw" / f"{title}_{engine}.png").write_bytes(png)
    im = key_out(png, k)
    im.save(out / f"{title}_{engine}.png")
    clear = (np.asarray(im)[..., 3] < 128).mean()
    return f"{title} {engine}: {im.width}x{im.height}, key {k}, {clear:.0%} transparent"


def main():
    out = Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT / "remaster"
    out.mkdir(parents=True, exist_ok=True)
    dump = Path(tempfile.mkdtemp())
    subprocess.run(["go", "run", "./cmd/viewdump", "-transparent", ".", str(dump)], cwd=ROOT, check=True)
    with ThreadPoolExecutor(12) as ex:
        jobs = [ex.submit(job, out, dump, t, s, d, e) for t, (s, d) in WALLS.items() for e in ("gemini", "openai")]
        for j in jobs:
            print(j.result(), flush=True)


if __name__ == "__main__":
    main()
