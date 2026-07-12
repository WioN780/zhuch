"""Death-position heatmap: 20x20 grid over the 2000x2000 map, per mode.

    python -m analytics.spark_jobs.death_heatmap --in out --out out_summary/death_heatmap.parquet
"""
from __future__ import annotations

import argparse
from pathlib import Path

from pyspark.sql import SparkSession
from pyspark.sql import functions as F

GRID = 20
MAP_SIZE = 2000.0
CELL = MAP_SIZE / GRID
RAMP = " .:-=+*#%@"


def compute(spark: SparkSession, parquet_root: str) -> list:
    df = spark.read.parquet(parquet_root)
    deaths = (
        df.filter(F.col("type") == "death")
        .withColumn("mode", F.get_json_object(F.col("data"), "$.mode"))
        .withColumn("gx", F.least(F.lit(GRID - 1), (F.col("pos.x") / CELL).cast("int")))
        .withColumn("gy", F.least(F.lit(GRID - 1), (F.col("pos.y") / CELL).cast("int")))
    )
    return (
        deaths.groupBy("mode", "gx", "gy").count()
        .orderBy("mode", "gy", "gx")
        .collect()
    )


def ascii_heatmap(rows, mode: str) -> str:
    grid = [[0] * GRID for _ in range(GRID)]
    for r in rows:
        if r["mode"] == mode:
            grid[r["gy"]][r["gx"]] = r["count"]
    peak = max((c for row in grid for c in row), default=0) or 1
    lines = []
    for row in grid:
        lines.append("".join(RAMP[min(len(RAMP) - 1, int(c / peak * (len(RAMP) - 1)))] for c in row))
    return "\n".join(lines)


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                  formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--in", dest="in_path", default="out")
    ap.add_argument("--out", default="out_summary/death_heatmap.parquet")
    ap.add_argument("--master", default="local[*]")
    args = ap.parse_args(argv)

    spark = SparkSession.builder.appName("zhuch-death-heatmap").master(args.master).getOrCreate()
    try:
        rows = compute(spark, args.in_path)
        modes = sorted({r["mode"] for r in rows})
        for mode in modes:
            print(f"\n{mode} ({MAP_SIZE:.0f}x{MAP_SIZE:.0f} map, {GRID}x{GRID} grid):")
            print(ascii_heatmap(rows, mode))
        out_path = Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_rows = [r.asDict() for r in rows]
        spark.createDataFrame(out_rows).write.mode("overwrite").parquet(str(out_path))
    finally:
        spark.stop()


if __name__ == "__main__":
    main()
