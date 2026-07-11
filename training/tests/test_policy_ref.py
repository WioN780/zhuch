import base64
import json
from pathlib import Path

import numpy as np
import pytest

from zhuch_train.policy_ref import forward

TESTDATA = Path(__file__).resolve().parents[2] / "testdata"


@pytest.mark.parametrize("name", ["golden_mlp_tiny.json", "golden_mlp_full.json"])
def test_golden(name):
    doc = json.loads((TESTDATA / name).read_text())
    flat = np.frombuffer(base64.b64decode(doc["weights_b64"]), dtype="<f4")
    out = forward(doc["sizes"], flat, np.asarray(doc["input"], dtype=np.float64))
    np.testing.assert_allclose(out, doc["expected_output"], atol=1e-6)
