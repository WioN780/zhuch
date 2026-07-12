"""Difficulty-tuning table: suggested bot stat multipliers derived from per-mode win rates.

Reads balance.py's summary Parquet (out_summary/balance.parquet) via pyarrow — no
Spark session needed here, it's a handful of rows, a simple table transform. Applies
a linear rule: push bots weaker when they win too much, stronger when too little,
targeting a 50% win rate.

    python -m analytics.spark_jobs.difficulty --summary out_summary/balance.parquet --out out_summary/difficulty.json
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path

import pyarrow.parquet as pq

TARGET_WIN_RATE = 0.5
# ponytail: proportional rule, not a PID controller — good enough for a nightly cron
# nudging stats a few % at a time. Add integral/derivative terms if multipliers
# oscillate instead of converging across nights.
GAIN = 0.5
MULTIPLIER_BOUNDS = (0.5, 1.5)


def suggest(bot_win_rate: float) -> float:
    error = TARGET_WIN_RATE - bot_win_rate  # bots winning too much -> negative -> weaken
    multiplier = 1.0 + GAIN * error
    lo, hi = MULTIPLIER_BOUNDS
    return round(min(hi, max(lo, multiplier)), 3)


def compute(summary_path: str) -> list[dict]:
    table = pq.read_table(summary_path)
    rows = [
        {
            "mode": row["mode"],
            "bot_win_rate": row["bot_win_rate"],
            "suggested_bot_stat_multiplier": suggest(row["bot_win_rate"]),
        }
        for row in table.to_pylist()
    ]
    return sorted(rows, key=lambda r: r["mode"])


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                  formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--summary", default="out_summary/balance.parquet", help="balance.py output Parquet")
    ap.add_argument("--out", default="out_summary/difficulty.json")
    args = ap.parse_args(argv)

    rows = compute(args.summary)
    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(rows, indent=2))
    print(json.dumps(rows, indent=2))


if __name__ == "__main__":
    main()
