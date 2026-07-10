"""Generate golden MLP vectors per docs/contracts.md section 5. Reproducible: python gen_golden.py"""
import base64
import hashlib
import json
import pathlib

import numpy as np

HERE = pathlib.Path(__file__).parent


def param_count(sizes):
    return sum(sizes[i + 1] * sizes[i] + sizes[i + 1] for i in range(len(sizes) - 1))


def forward(sizes, flat, x):
    # float64 math, layer-major flat layout: W (out,in) row-major then bias
    a = x.astype(np.float64)
    off = 0
    for i in range(len(sizes) - 1):
        nin, nout = sizes[i], sizes[i + 1]
        w = flat[off:off + nout * nin].astype(np.float64).reshape(nout, nin)
        off += nout * nin
        b = flat[off:off + nout].astype(np.float64)
        off += nout
        a = w @ a + b
        if i < len(sizes) - 2:
            a = np.tanh(a)
    return a


def make(sizes, seed, name):
    rng = np.random.default_rng(seed)
    flat = (rng.standard_normal(param_count(sizes)) * 0.5).astype(np.float32)
    x = rng.standard_normal(sizes[0]).astype(np.float32)
    out = forward(sizes, flat, x)
    raw = flat.tobytes()  # little-endian float32
    doc = {
        "sizes": sizes,
        "sha256": hashlib.sha256(raw).hexdigest(),
        "weights_b64": base64.b64encode(raw).decode(),
        "input": [float(v) for v in x],
        "expected_output": [float(v) for v in out],
    }
    (HERE / name).write_text(json.dumps(doc, indent=1))
    print(name, "params:", param_count(sizes), "output:", out)


if __name__ == "__main__":
    make([3, 4, 2], seed=7, name="golden_mlp_tiny.json")
    make([82, 64, 64, 5], seed=42, name="golden_mlp_full.json")
