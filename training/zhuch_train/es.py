"""OpenAI-ES (Salimans 2017): mirrored sampling + common random numbers,
centered-rank shaping, Adam on the center. Shares the eval harness with ga.py."""
from __future__ import annotations

from dataclasses import asdict
from pathlib import Path

import numpy as np

from .arena_client import ArenaClient
from .ga import (Config, evaluate_population, gauntlet, load_checkpoint, make_plans,
                 save_checkpoint, _restore_rng)
from .genome import Genome, param_count
from .league import League, BASELINE_ELO
from .tracking import Tracker


def run_es(cfg: Config) -> dict:
    client = ArenaClient(cfg.arena_urls)
    tracker = Tracker(cfg.run_name, asdict(cfg))
    n_params = param_count()
    half = max(1, cfg.pop // 2)
    n = 2 * half

    if cfg.resume:
        state, arrays, league = load_checkpoint(Path(cfg.resume))
        rng = _restore_rng(state)
        theta = arrays["theta"].astype(np.float64)
        m, v = arrays["adam_m"].astype(np.float64), arrays["adam_v"].astype(np.float64)
        t, start_gen = state["adam_t"], state["gen"]
        best_elo, best_fit_ever = state["best_elo"], state["best_fit"]
    else:
        rng = np.random.default_rng(cfg.seed)
        league = League()
        theta = Genome.he_init(rng).weights.astype(np.float64)
        m = np.zeros(n_params)
        v = np.zeros(n_params)
        t, start_gen, best_elo, best_fit_ever = 0, 0, BASELINE_ELO, -np.inf

    b1, b2, adam_eps = 0.9, 0.999, 1e-8
    for gen in range(start_gen, cfg.generations):
        master = int(rng.integers(2 ** 63))
        eps = rng.standard_normal((half, n_params))
        genomes = []
        for e in eps:
            genomes.append(Genome((theta + cfg.sigma * e).astype("<f4")))
            genomes.append(Genome((theta - cfg.sigma * e).astype("<f4")))
        # common random numbers: each mirrored pair shares seeds + opponent draws
        plans = [p for p in make_plans(half, master, league, cfg) for _ in range(2)]
        fits = evaluate_population(genomes, plans, client, cfg)

        ranks = np.empty(n)
        ranks[np.argsort(fits)] = np.arange(n)
        shaped = ranks / (n - 1) - 0.5 if n > 1 else np.zeros(n)
        grad = ((shaped[0::2, None] - shaped[1::2, None]) * eps).sum(axis=0) / (n * cfg.sigma)

        t += 1
        m = b1 * m + (1 - b1) * grad
        v = b2 * v + (1 - b2) * grad * grad
        theta = theta + cfg.lr * (m / (1 - b1 ** t)) / (np.sqrt(v / (1 - b2 ** t)) + adam_eps)

        center = Genome(theta.astype("<f4"))
        best_fit_ever = max(best_fit_ever, float(fits.max()))
        metrics = {"fit_mean": float(fits.mean()), "fit_max": float(fits.max()),
                   "fit_best_ever": best_fit_ever, "elo_best": best_elo}
        tracker.log_metrics(metrics, step=gen)
        print(f"[es gen {gen}] fit mean={metrics['fit_mean']:.1f} max={metrics['fit_max']:.1f} "
              f"best_ever={best_fit_ever:.1f} elo={best_elo:.0f}", flush=True)

        if (gen + 1) % cfg.gauntlet_every == 0:
            best_elo = gauntlet(center, best_elo, league, client, cfg, int(rng.integers(2 ** 63)))
            if best_elo > league.max_elo() + cfg.hof_margin:
                league.snapshot(center, best_elo, gen)
            tracker.log_metrics({"elo_best": best_elo}, step=gen)

        if (gen + 1) % cfg.checkpoint_every == 0 or gen == cfg.generations - 1:
            save_checkpoint(cfg, gen + 1, rng, league, center, best_elo, best_fit_ever,
                            {"theta": theta, "adam_m": m, "adam_v": v}, extra={"adam_t": t})
            tracker.log_artifact(cfg.run_dir / "best.json")

    tracker.close()
    return {"best_fitness": best_fit_ever, "best_elo": best_elo, "run_dir": cfg.run_dir}
