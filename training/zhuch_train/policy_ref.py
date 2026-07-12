"""Reference MLP forward pass — must match testdata/golden_mlp_*.json (contracts sections 3, 5)."""
import numpy as np


def forward(sizes, flat, x):
    """float64 math; flat layout per layer: W(out,in) row-major then bias; tanh hidden, identity out."""
    a = np.asarray(x, dtype=np.float64)
    flat = np.asarray(flat)
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
