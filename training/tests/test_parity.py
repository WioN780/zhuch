"""Real-arena parity: skipped unless ARENA_URL is set (the Go arena is built separately)."""
import base64
import json
import os
from pathlib import Path

import numpy as np
import pytest

from zhuch_train.arena_client import ArenaClient
from zhuch_train.policy_ref import forward

TESTDATA = Path(__file__).resolve().parents[2] / "testdata"

pytestmark = pytest.mark.skipif(not os.environ.get("ARENA_URL"),
                                reason="ARENA_URL not set")


@pytest.mark.parametrize("name", ["golden_mlp_tiny.json", "golden_mlp_full.json"])
def test_forward_parity(name):
    doc = json.loads((TESTDATA / name).read_text())
    client = ArenaClient([os.environ["ARENA_URL"]])
    got = client.forward(doc["sizes"], doc["weights_b64"], doc["input"])
    flat = np.frombuffer(base64.b64decode(doc["weights_b64"]), dtype="<f4")
    ref = forward(doc["sizes"], flat, np.asarray(doc["input"], dtype=np.float64))
    np.testing.assert_allclose(got, ref, atol=1e-5)
    np.testing.assert_allclose(got, doc["expected_output"], atol=1e-5)
