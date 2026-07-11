# Databricks notebook source
# MAGIC %md
# MAGIC # zhuch bot-balance report
# MAGIC
# MAGIC Databricks-notebook port of `analytics/spark_jobs/balance.py`. To import: Databricks
# MAGIC Free Edition -> Workspace -> Import -> upload this file (the `# Databricks notebook
# MAGIC source` header and `# COMMAND ----------` markers make Databricks render it as cells
# MAGIC automatically). Attach it to a cluster/serverless compute and run all.
# MAGIC
# MAGIC Before running: upload the Parquet tree written by `analytics/sink.py` (drag the
# MAGIC `out/` folder into a Unity Catalog volume, or `dbfs:/FileStore/...` on older
# MAGIC workspaces) and point `PARQUET_PATH` below at it. `spark` and `display` are provided
# MAGIC by the notebook runtime — no `SparkSession.builder` needed, that's the only real
# MAGIC difference from the local job.

# COMMAND ----------

PARQUET_PATH = "/Volumes/main/zhuch/telemetry/out"  # <- change me after upload

# COMMAND ----------

import json

from pyspark.sql import functions as F

df = spark.read.parquet(PARQUET_PATH)
df = df.withColumn("mode", F.get_json_object(F.col("data"), "$.mode"))

# COMMAND ----------

kill_rows = (
    df.filter(F.col("type") == "kill")
    .withColumn("is_bot", F.col("actor").startswith("bot"))
    .groupBy("mode", "is_bot").count()
    .collect()
)
kills_by_mode = {}
for r in kill_rows:
    kills_by_mode.setdefault(r["mode"], {"bot": 0, "human": 0})
    kills_by_mode[r["mode"]]["bot" if r["is_bot"] else "human"] = r["count"]

# COMMAND ----------

# ponytail: episode_end rows are one per episode (small), so parsing the nested
# tanks[] JSON in Python beats hand-building a Spark struct schema for a one-shot
# winner pick — same call as the local job in analytics/spark_jobs/balance.py.
end_rows = df.filter(F.col("type") == "episode_end").select("mode", "data").collect()
stats = {}
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
rows.sort(key=lambda r: r["mode"])

# COMMAND ----------

result_df = spark.createDataFrame(rows)
display(result_df)  # Databricks' rich table/chart widget

# COMMAND ----------

# Optional: persist the summary alongside the raw data.
# result_df.write.mode("overwrite").parquet(PARQUET_PATH + "_summary/balance")
