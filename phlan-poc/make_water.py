#!/usr/bin/env python3
"""New Phlan PoC: the harbour water the engine copy draws as a surface
(blobber/assets/walls/floor_phlan_water.png, 512x512, tiles both ways).
One OpenAI image, then made tileable: blended with itself shifted by half,
weighted towards the original in the middle and the shifted copy at the edges."""
import base64, json, urllib.request
from pathlib import Path
import numpy as np
from PIL import Image
import io

HERE = Path(__file__).resolve().parent
KEY = (Path.home() / ".config/blobber/openai_key").read_text().strip()
PROMPT = ("Seamless tileable texture, seen straight from above: the deep blue-green water of a medieval "
          "harbour on the Moonsea (Forgotten Realms), gentle small ripples and wavelets with soft light "
          "glints, slightly darker deep tones, no foam lines, no objects, no boats, no shore, no horizon, "
          "even lighting over the whole image, painterly high-fantasy game texture with modern detail.")


def main():
    req = urllib.request.Request("https://api.openai.com/v1/images/generations",
                                 json.dumps({"model": "gpt-image-2", "prompt": PROMPT, "size": "1024x1024",
                                             "n": 1}).encode(),
                                 {"Content-Type": "application/json", "Authorization": f"Bearer {KEY}"})
    with urllib.request.urlopen(req, timeout=600) as r:
        png = base64.b64decode(json.load(r)["data"][0]["b64_json"])
    (HERE / "raw").mkdir(exist_ok=True)
    (HERE / "raw/water_openai.png").write_bytes(png)
    tile(png)


def tile(png):
    a = np.asarray(Image.open(io.BytesIO(png)).convert("RGB"), dtype=np.float32)
    # graded by brightness onto the original's harbour blues (the model's water came out near black)
    l = a @ np.array([0.299, 0.587, 0.114], dtype=np.float32)
    l = (l - np.percentile(l, 1)) / (np.percentile(l, 99.5) - np.percentile(l, 1))
    ramp = np.array([[16, 52, 110], [34, 92, 160], [70, 140, 200], [215, 235, 250]], dtype=np.float32)
    x = np.clip(l, 0, 1) * (len(ramp) - 1)
    i = np.minimum(x.astype(int), len(ramp) - 2)
    a = ramp[i] + (ramp[i + 1] - ramp[i]) * (x - i)[..., None]
    n = a.shape[0]
    b = np.roll(a, (n // 2, n // 2), axis=(0, 1))
    t = np.abs(np.linspace(-1, 1, n))
    w = (1 - np.maximum(t[:, None], t[None, :])) ** 0.5  # 1 in the middle, 0 at the edges
    out = a * w[..., None] + b * (1 - w[..., None])
    Image.fromarray(out.astype(np.uint8)).resize((512, 512), Image.LANCZOS).save(
        HERE / "blobber/assets/walls/floor_phlan_water.png")


if __name__ == "__main__":
    import sys
    if sys.argv[1:] == ["tile"]:  # regrade raw/water_openai.png only
        tile((HERE / "raw/water_openai.png").read_bytes())
    else:
        main()
