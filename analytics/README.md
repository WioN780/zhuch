# zhuch analytics pipeline (WS-G)

Kafka -> Parquet -> Spark -> Airflow pipeline for telemetry events (`docs/contracts.md`
§6, topic `zhuch.events.v1`). Built against a synthetic event generator so it works
standalone before the Go producers are wired up — they gate emission on
`KAFKA_BROKERS`, same convention `analytics/synthetic.py` follows here.

## Pipeline

```
 [synthetic.py]  (or the Go engine, later)
        |  publish (kafka-python-ng)
        v
   Kafka topic: zhuch.events.v1        (redpanda, infra/compose/docker-compose.yml)
        |
        v
   [sink.py]  batched consumer
        |  flush every N events / T seconds
        v
   Parquet tree: out/dt=YYYY-MM-DD/hour=HH/part-*.parquet
        |
        +--> spark_jobs/balance.py        per-mode episodes/kills/win-rate table
        +--> spark_jobs/death_heatmap.py  20x20 death-position grid per mode
        +--> spark_jobs/difficulty.py     reads balance's output; no Spark needed
        |
        v
   infra/airflow/dags/nightly_retrain.py
   (03:00 nightly: balance report -> gate -> conditional retrain -> gauntlet -> promote)
```

## Quickstart — no Docker (NDJSON, fastest loop)

```powershell
cd analytics
python -m venv .venv
.venv\Scripts\pip install -e .[dev]

.venv\Scripts\python -m analytics.synthetic --episodes 200 --seed 1 --out events.ndjson
.venv\Scripts\python -m analytics.sink --from-ndjson events.ndjson --out out
```

Parquet lands under `out/dt=.../hour=.../part-*.parquet`.

## Quickstart — with Docker (real Kafka via redpanda)

```powershell
cd infra/compose
docker compose up -d redpanda

cd ../../analytics
.venv\Scripts\python -m analytics.synthetic --episodes 200 --brokers localhost:19092
.venv\Scripts\python -m analytics.sink --brokers localhost:19092 --out out
```

## Spark jobs

```powershell
.venv\Scripts\pip install -e .[spark]   # pyspark; local[*], no cluster needed
.venv\Scripts\python -m analytics.spark_jobs.balance --in out --out out_summary/balance.parquet
.venv\Scripts\python -m analytics.spark_jobs.death_heatmap --in out --out out_summary/death_heatmap.parquet
.venv\Scripts\python -m analytics.spark_jobs.difficulty --summary out_summary/balance.parquet --out out_summary/difficulty.json
```

`difficulty.py` deliberately doesn't start a Spark session — it just reads
`balance.py`'s small summary table with pyarrow.

Recent pyspark generally runs `local[*]` on native Windows without winutils.exe, but
if it still trips on a Windows-specific Hadoop-native error, run the Spark jobs from
WSL instead. `infra/airflow/` has the same caveat for a different reason: it runs
Linux containers, so any venv it shells out to must be a Linux venv built from WSL,
not the Windows `.venv` used for the local loop above.

## Honest note: DuckDB would suffice here

At synthetic-event volumes (a few hundred episodes, low tens of MB of Parquet),
`duckdb.sql("select * from read_parquet('out/**/*.parquet')")` answers every query in
`balance.py` / `death_heatmap.py` in-process, no JVM, in milliseconds. Spark earns its
keep once the Parquet tree stops fitting comfortably on one machine — real production
telemetry at thousands of concurrent rooms, weeks of retention, or jobs that need to
join against other large tables (match history, player profiles). Until then, this is
deliberately built with the distributed-compute path proven out ahead of needing it;
swap `SparkSession.builder...getOrCreate()` for `duckdb.connect()` in `spark_jobs/*.py`
if JVM startup ever becomes the bottleneck instead of the analysis itself.

## Spark on this machine

The Windows host blocks JVM loopback socket pairs (security software), so PySpark cannot start locally — run Spark jobs from WSL instead:

```bash
wsl -d Ubuntu -- bash -c "cd /mnt/d/Random\ Projects/zhuch && JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64 ~/.zhuch-spark/bin/python -m analytics.spark_jobs.balance --in analytics/out --out analytics/out_summary/balance.parquet"
```
