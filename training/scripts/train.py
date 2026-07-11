"""Train a zhuch bot: python scripts/train.py --algo ga --generations 100 --arena-urls http://127.0.0.1:8081"""
from __future__ import annotations

import argparse
import time
from pathlib import Path

from zhuch_train.ga import Config, run_ga
from zhuch_train.es import run_es


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--algo", choices=["ga", "es"], default="ga")
    ap.add_argument("--generations", type=int, default=100)
    ap.add_argument("--arena-urls", nargs="+", default=["http://127.0.0.1:8081"])
    ap.add_argument("--pop", type=int, default=256)
    ap.add_argument("--sigma", type=float, default=None, help="default: 0.03 ga, 0.05 es")
    ap.add_argument("--lr", type=float, default=0.01, help="es Adam learning rate")
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--max-ticks", type=int, default=1800)
    ap.add_argument("--run-name", default=None)
    ap.add_argument("--resume", default=None, help="runs/<name> checkpoint dir to resume")
    args = ap.parse_args()

    sigma = args.sigma if args.sigma is not None else (0.03 if args.algo == "ga" else 0.05)
    if args.resume:
        run_dir = Path(args.resume)
        runs_dir, run_name = run_dir.parent, run_dir.name
    else:
        runs_dir = Path("runs")
        run_name = args.run_name or f"{args.algo}-{time.strftime('%Y%m%d-%H%M%S')}"

    cfg = Config(algo=args.algo, generations=args.generations, pop=args.pop, sigma=sigma,
                 lr=args.lr, arena_urls=args.arena_urls, run_name=run_name, runs_dir=runs_dir,
                 seed=args.seed, max_ticks=args.max_ticks, resume=args.resume)
    result = (run_ga if args.algo == "ga" else run_es)(cfg)
    print(f"done: best_fitness={result['best_fitness']:.2f} best_elo={result['best_elo']:.1f} "
          f"checkpoint={result['run_dir']}")


if __name__ == "__main__":
    main()
