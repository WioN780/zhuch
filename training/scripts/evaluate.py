"""Gauntlet a model vs the scripted baseline; exit 0 iff Elo >= --gate (CI/MLflow promotion gate).

python scripts/evaluate.py --model runs/x/best.json --arena-urls http://127.0.0.1:8081 --gate 1250
"""
from __future__ import annotations

import argparse
import sys

import numpy as np

from zhuch_train.arena_client import ArenaClient
from zhuch_train.fitness import fitness
from zhuch_train.genome import Genome
from zhuch_train.ga import _episode_score, _job, Config
from zhuch_train.league import BASELINE_ELO, elo_update


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--model", required=True)
    ap.add_argument("--arena-urls", nargs="+", default=["http://127.0.0.1:8081"])
    ap.add_argument("--episodes", type=int, default=50)
    ap.add_argument("--max-ticks", type=int, default=1800)
    ap.add_argument("--seed", type=int, default=1234)
    ap.add_argument("--gate", type=float, default=1250.0)
    args = ap.parse_args()

    genome = Genome.load(args.model)
    client = ArenaClient(args.arena_urls)
    cfg = Config(max_ticks=args.max_ticks)

    seeds = [int(np.random.default_rng(ss).integers(2 ** 31))
             for ss in np.random.SeedSequence(args.seed).spawn(args.episodes)]
    jobs = [_job(genome, [{"scripted": True}], s, cfg) for s in seeds]
    results = client.eval_batch(jobs)

    elo = BASELINE_ELO
    wins = losses = draws = 0
    fits = []
    for r in results:
        score = _episode_score(r)
        wins += score == 1.0
        losses += score == 0.0
        draws += score == 0.5
        elo, _ = elo_update(elo, BASELINE_ELO, score)  # opponent anchored at 1200
        fits.append(fitness({**r["tanks"][0], "ticks": r["ticks"]}, args.max_ticks))

    print(f"episodes={args.episodes} W/L/D={wins}/{losses}/{draws} "
          f"mean_fitness={np.mean(fits):.2f} elo={elo:.1f} gate={args.gate}")
    sys.exit(0 if elo >= args.gate else 1)


if __name__ == "__main__":
    main()
