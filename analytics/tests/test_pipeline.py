"""Offline end-to-end: synthetic -> NDJSON -> sink --from-ndjson -> Parquet.

The balance-job test additionally requires pyspark and is skipped otherwise
(see analytics/README.md for the Windows/WSL note on pyspark local[*]).
"""
from __future__ import annotations

import os
import shutil
import sys

import pyarrow.dataset as ds
import pytest

from analytics import sink, synthetic
from analytics.sink import SCHEMA


def test_synthetic_ndjson_sink_parquet(tmp_path):
    ndjson_path = tmp_path / "events.ndjson"
    out_dir = tmp_path / "out"

    synthetic.main(["--episodes", "200", "--seed", "42", "--out", str(ndjson_path),
                     "--spread-hours", "6"])
    assert ndjson_path.exists()
    n_lines = sum(1 for _ in ndjson_path.open(encoding="utf-8"))
    assert n_lines > 0

    sink.main(["--from-ndjson", str(ndjson_path), "--out", str(out_dir), "--batch-size", "500"])

    parquet_files = list(out_dir.glob("dt=*/hour=*/part-*.parquet"))
    assert parquet_files
    hours = {p.parent.name for p in parquet_files}
    assert len(hours) >= 2, "expected multiple hour partitions given --spread-hours 6"

    table = ds.dataset(str(out_dir), format="parquet", partitioning="hive").to_table()
    assert table.num_rows == n_lines
    for field in SCHEMA.names:
        assert field in table.schema.names


# Windows: Spark's local-mode JVM pipe setup breaks on unsupported JDKs (seen with
# JDK 25: PipeImpl -> UnixDomainSockets "Invalid argument: connect"). WSL is the
# supported path — see analytics/README.md. Opt in anyway with ZHUCH_SPARK_TEST=1.
@pytest.mark.skipif(
    sys.platform == "win32" and not os.environ.get("ZHUCH_SPARK_TEST"),
    reason="pyspark local[*] unsupported on this Windows JDK; run from WSL",
)
def test_balance_job_smoke(tmp_path):
    pytest.importorskip("pyspark")
    if not (shutil.which("java") or os.environ.get("JAVA_HOME")):
        pytest.skip("no JVM on PATH — run Spark jobs from WSL (see analytics/README.md)")
    from pyspark.sql import SparkSession

    from analytics.spark_jobs import balance

    ndjson_path = tmp_path / "events.ndjson"
    out_dir = tmp_path / "out"
    synthetic.main(["--episodes", "60", "--seed", "3", "--out", str(ndjson_path)])
    sink.main(["--from-ndjson", str(ndjson_path), "--out", str(out_dir)])

    spark = SparkSession.builder.appName("test-balance").master("local[1]").getOrCreate()
    try:
        rows = balance.compute(spark, str(out_dir))
    finally:
        spark.stop()

    assert rows
    for r in rows:
        assert r["episodes"] > 0
        assert 0.0 <= r["bot_win_rate"] <= 1.0
        assert r["kills_bot"] + r["kills_human"] >= 0
