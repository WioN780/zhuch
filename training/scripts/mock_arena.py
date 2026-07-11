"""Stdlib mock of the Go arena (contracts section 4): /eval, /eval_batch, /forward, /healthz.

Eval stats are canned but plausible and a deterministic function of
(seed, candidate weights), so training against it is reproducible.
/forward uses zhuch_train.policy_ref so parity logic is testable offline.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import numpy as np

from zhuch_train.policy_ref import forward


def eval_one(job: dict) -> dict:
    seed = int(job.get("seed", 0))
    max_ticks = int(job.get("max_ticks", 1800))
    tanks = job["tanks"]
    h = hashlib.sha256(seed.to_bytes(8, "little", signed=True)
                       + tanks[0].get("weights_b64", "").encode()).digest()
    rng = np.random.default_rng(int.from_bytes(h[:8], "little"))
    cand_dead = rng.random() < 0.4
    ticks = int(rng.integers(200, max_ticks + 1)) if cand_dead else max_ticks
    out = []
    for i, t in enumerate(tanks):
        if i == 0:
            alive = not cand_dead
            death = ticks if cand_dead else -1  # episode ends when candidate dies
        else:
            alive = bool(rng.random() < 0.5)
            death = int(rng.integers(1, ticks + 1)) if not alive else -1
        out.append({"name": t.get("name", f"t{i}"), "score": int(rng.integers(0, 400)),
                    "kills": int(rng.integers(0, 3)), "death_tick": death, "alive": alive})
    return {"ticks": ticks, "tanks": out}


class Handler(BaseHTTPRequestHandler):
    def _send(self, code: int, body: bytes, ctype: str = "application/json") -> None:
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path == "/healthz":
            self._send(200, b"ok", "text/plain")
        else:
            self._send(404, b"{}")

    def do_POST(self) -> None:
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))))
        if self.path == "/eval":
            resp = eval_one(body)
        elif self.path == "/eval_batch":
            resp = {"results": [eval_one(j) for j in body["jobs"]]}
        elif self.path == "/forward":
            flat = np.frombuffer(base64.b64decode(body["weights_b64"]), dtype="<f4")
            out = forward(body["sizes"], flat, np.asarray(body["input"], dtype=np.float64))
            resp = {"output": [float(v) for v in out]}
        else:
            self._send(404, b"{}")
            return
        self._send(200, json.dumps(resp).encode())

    def log_message(self, *args) -> None:  # quiet
        pass


def make_server(port: int = 0) -> ThreadingHTTPServer:
    return ThreadingHTTPServer(("127.0.0.1", port), Handler)


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--port", type=int, default=8081)
    args = ap.parse_args()
    srv = make_server(args.port)
    print(f"mock arena listening on http://127.0.0.1:{srv.server_address[1]}", flush=True)
    srv.serve_forever()
