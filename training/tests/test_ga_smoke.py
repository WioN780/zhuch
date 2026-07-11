import sys
import threading
from pathlib import Path

import numpy as np
import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
import mock_arena  # noqa: E402

from zhuch_train.es import run_es  # noqa: E402
from zhuch_train.ga import Config, run_ga  # noqa: E402


@pytest.fixture(scope="module")
def arena_url():
    srv = mock_arena.make_server(0)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    yield f"http://127.0.0.1:{srv.server_address[1]}"
    srv.shutdown()


def _cfg(arena_url, tmp_path, algo):
    return Config(algo=algo, generations=2, pop=8, arena_urls=[arena_url],
                  run_name=f"smoke-{algo}", runs_dir=tmp_path,
                  gauntlet_every=1, gauntlet_episodes=6)  # exercise gauntlet/league too


def _check(result, tmp_path, algo):
    assert np.isfinite(result["best_fitness"])
    d = tmp_path / f"smoke-{algo}"
    for f in ("state.json", "arrays.npz", "best.json"):
        assert (d / f).exists()
    assert (d / "league" / "league.json").exists()


def test_ga_smoke(arena_url, tmp_path):
    _check(run_ga(_cfg(arena_url, tmp_path, "ga")), tmp_path, "ga")


def test_es_smoke(arena_url, tmp_path):
    _check(run_es(_cfg(arena_url, tmp_path, "es")), tmp_path, "es")


def test_ga_resume(arena_url, tmp_path):
    cfg = _cfg(arena_url, tmp_path, "ga")
    run_ga(cfg)
    cfg2 = _cfg(arena_url, tmp_path, "ga")
    cfg2.generations = 3
    cfg2.resume = str(tmp_path / "smoke-ga")
    result = run_ga(cfg2)
    assert np.isfinite(result["best_fitness"])
