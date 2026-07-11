"""Per-mode balance report: episodes, kills bot vs human, mean episode length, bot win rate.

Reads the hourly-partitioned Parquet tree written by analytics/sink.py (Spark
auto-discovers the dt=/hour= Hive-style partitions) and writes a one-row-per-mode
summary Parquet that analytics/spark_jobs/difficulty.py consumes downstream.

    python -m analytics.spark_jobs.balance --in out --out out_summary/balance.parquet
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path

from pyspark.sql import SparkSession
from pyspark.sql import functions as F


def compute(spark: SparkSession, parquet_root: str) -> list[dict]:
    df = spark.read.parquet(parquet_root)
    df = df.withColumn("mode", F.get_json_object(F.col("data"), "$.mode"))

    kill_rows = (
        df.filter(F.col("type") == "kill")
        .withColumn("is_bot", F.col("actor").startswith("bot"))
        .groupBy("mode", "is_bot").count()
        .collect()
    )
    kills_by_mode: dict[str, dict[str, int]] = {}
    for r in kill_rows:
        kills_by_mode.setdefault(r["mode"], {"bot": 0, "human": 0})
        kills_by_mode[r["mode"]]["bot" if r["is_bot"] else "human"] = r["count"]

    # ponytail: episode_end rows are one per episode (small, bounded by --episodes),
    # so parsing the nested tanks[] JSON in Python beats hand-building a Spark
    # struct schema just for a one-shot winner pick. Kill counts above go through
    # a real Spark aggregation since that's the column that can actually get big.
    end_rows = df.filter(F.col("type") == "episode_end").select("mode", "data").collect()
    stats: dict[str, dict] = {}
    for r in end_rows:
        mode = r["mode"]
        d = json.loads(r["data"])
        s = stats.setdefault(mode, {"episodes": 0, "ticks_sum": 0, "bot_wins": 0})
        s["episodes"] += 1
        s["ticks_sum"] += d.get("ticks", 0)
        tanks = d.get("tanks", [])
        if tanks:
            winner = max(tanks, key=lambda t: t.get("score", 0))
            s["bot_wins"] += winner["name"].startswith("bot")

    rows = []
    for mode, s in stats.items():
        k = kills_by_mode.get(mode, {"bot": 0, "human": 0})
        rows.append({
            "mode": mode,
            "episodes": s["episodes"],
            "kills_bot": k["bot"],
            "kills_human": k["human"],
            "mean_episode_ticks": s["ticks_sum"] / s["episodes"] if s["episodes"] else 0.0,
            "bot_win_rate": s["bot_wins"] / s["episodes"] if s["episodes"] else 0.0,
        })
    return sorted(rows, key=lambda r: r["mode"])


def _print_table(rows: list[dict]) -> None:
    cols = ["mode", "episodes", "kills_bot", "kills_human", "mean_episode_ticks", "bot_win_rate"]

    def fmt(v):
        return f"{v:.2f}" if isinstance(v, float) else str(v)

    cells = [[fmt(r[c]) for c in cols] for r in rows]
    widths = [max(len(c), *(len(row[i]) for row in cells)) if cells else len(c)
              for i, c in enumerate(cols)]
    print("  ".join(c.ljust(w) for c, w in zip(cols, widths)))
    print("-" * (sum(widths) + 2 * (len(cols) - 1)))
    for row in cells:
        print("  ".join(v.ljust(w) for v, w in zip(row, widths)))


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                  formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--in", dest="in_path", default="out", help="Parquet tree root")
    ap.add_argument("--out", default="out_summary/balance.parquet")
    ap.add_argument("--master", default="local[*]")
    args = ap.parse_args(argv)

    spark = SparkSession.builder.appName("zhuch-balance").master(args.master).getOrCreate()
    try:
        rows = compute(spark, args.in_path)
        _print_table(rows)
        out_path = Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        spark.createDataFrame(rows).write.mode("overwrite").parquet(str(out_path))
    finally:
        spark.stop()


if __name__ == "__main__":
    main()
