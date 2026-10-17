#!/usr/bin/env python3
"""Tiny local web UI for tools/flux.sh.
  python3 tools/flux-web.py [port]   ->  http://localhost:7860
Images land in ~/Pictures/flux (FLUX_OUT to change). One generation at a time."""
import base64, json, os, subprocess, sys, tempfile, threading, time
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from pathlib import Path

FLUX = Path(__file__).with_name("flux.sh")
OUT = Path(os.environ.get("FLUX_OUT", Path.home() / "Pictures/flux"))
OUT.mkdir(parents=True, exist_ok=True)
LOCK = threading.Lock()

PAGE = """<!doctype html><meta charset=utf-8><title>Flux Klein</title>
<meta name=viewport content="width=device-width,initial-scale=1">
<style>
body{font:15px system-ui;margin:0;padding:16px;background:#16161a;color:#eee;max-width:1100px;margin:auto}
textarea,input,select,button{font:inherit;background:#24242b;color:#eee;border:1px solid #444;border-radius:6px;padding:6px}
textarea{width:100%;box-sizing:border-box;height:80px}
.row{display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin:8px 0}
button{background:#5b5bd6;border:0;padding:8px 18px;cursor:pointer}button:disabled{opacity:.5}
#out img{max-width:100%;border-radius:6px}
#gal{display:grid;grid-template-columns:repeat(auto-fill,minmax(140px,1fr));gap:8px;margin-top:16px}
#gal img{width:100%;border-radius:4px;cursor:pointer}
</style>
<h2>Flux Klein (local)</h2>
<textarea id=p placeholder="prompt"></textarea>
<div class=row>
 <select id=s><option>1024x1024<option>768x768<option>512x512<option>1024x768<option>768x1024<option>1536x512</select>
 steps <input id=st type=number value=4 min=1 max=50 style=width:60px>
 seed <input id=sd placeholder=random style=width:90px>
 <select id=m><option>flux2-klein-4b<option>flux2-klein-9b</select>
 refs <input id=r type=file accept=image/* multiple>
 <button id=go>Generate</button> <span id=st2></span>
</div>
<div id=out></div><div id=gal></div>
<script>
const $=id=>document.getElementById(id);
const b64=f=>new Promise(r=>{const x=new FileReader();x.onload=()=>r(x.result.split(',')[1]);x.readAsDataURL(f)});
async function gal(){const l=await (await fetch('/list')).json();
 $('gal').innerHTML=l.map(n=>`<a href="/img/${n}" target=_blank><img loading=lazy src="/img/${n}" title="${n}"></a>`).join('')}
$('go').onclick=async()=>{
 $('go').disabled=true;const t0=Date.now();const iv=setInterval(()=>$('st2').textContent=`working… ${((Date.now()-t0)/1000)|0}s`,1000);
 try{const refs=await Promise.all([...$('r').files].map(b64));
  const res=await fetch('/gen',{method:'POST',body:JSON.stringify({prompt:$('p').value,size:$('s').value,steps:$('st').value,seed:$('sd').value,model:$('m').value,refs})});
  const j=await res.json();
  $('out').innerHTML=j.file?`<img src="/img/${j.file}">`:`<pre>${j.error}</pre>`;gal();
 }finally{clearInterval(iv);$('go').disabled=false;$('st2').textContent=''}};
gal();
</script>"""


class H(BaseHTTPRequestHandler):
    def send(self, code, body, ctype):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/":
            return self.send(200, PAGE.encode(), "text/html")
        if self.path == "/list":
            files = sorted(OUT.glob("*.png"), key=lambda p: p.stat().st_mtime, reverse=True)
            return self.send(200, json.dumps([f.name for f in files]).encode(), "application/json")
        if self.path.startswith("/img/"):
            f = OUT / Path(self.path[5:]).name
            if f.is_file():
                return self.send(200, f.read_bytes(), "image/png")
        self.send(404, b"not found", "text/plain")

    def do_POST(self):
        req = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        name = time.strftime("%Y%m%d-%H%M%S") + ".png"
        with tempfile.TemporaryDirectory() as tmp, LOCK:
            refs = []
            for i, data in enumerate(req.get("refs") or []):
                p = Path(tmp, f"ref{i}.png")
                p.write_bytes(base64.b64decode(data))
                refs.append(str(p))
            env = dict(os.environ, FLUX_MODEL=req["model"], FLUX_STEPS=str(req["steps"]))
            if req.get("seed"):
                env["FLUX_SEED"] = str(req["seed"])
            r = subprocess.run([str(FLUX), req["prompt"], str(OUT / name), req["size"], *refs],
                               env=env, capture_output=True, text=True)
        body = {"file": name} if r.returncode == 0 else {"error": (r.stdout + r.stderr)[-3000:]}
        self.send(200, json.dumps(body).encode(), "application/json")


port = int(sys.argv[1]) if len(sys.argv) > 1 else 7860
print(f"http://localhost:{port}  (images -> {OUT})")
ThreadingHTTPServer(("127.0.0.1", port), H).serve_forever()
