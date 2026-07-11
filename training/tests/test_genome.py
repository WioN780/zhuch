import numpy as np
import pytest

from zhuch_train.genome import Genome, param_count


def test_param_count():
    assert param_count() == 9797  # contracts section 3


def test_model_json_roundtrip(tmp_path):
    g = Genome.he_init(np.random.default_rng(0))
    doc = g.to_model_json()
    assert doc["format"] == "zhuch-mlp" and doc["version"] == 1
    g2 = Genome.from_model_json(doc)
    assert np.array_equal(g.weights, g2.weights)
    assert g.sizes == g2.sizes

    p = tmp_path / "m.json"
    g.save(p)
    assert np.array_equal(Genome.load(p).weights, g.weights)


def test_sha_check():
    doc = Genome.he_init(np.random.default_rng(1)).to_model_json()
    doc["sha256"] = "0" * 64
    with pytest.raises(ValueError, match="sha256"):
        Genome.from_model_json(doc)


def test_mutate_changes_nearly_all_genes():
    rng = np.random.default_rng(2)
    g = Genome.he_init(rng)
    before = g.weights.copy()
    child = g.gaussian_mutate(rng, sigma=0.03)
    assert np.array_equal(g.weights, before)  # parent untouched
    changed = np.mean(child.weights != g.weights)
    assert changed > 0.99
