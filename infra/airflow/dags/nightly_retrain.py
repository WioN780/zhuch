"""Nightly retrain pipeline: balance report -> Elo gate -> conditional retrain ->
gauntlet -> MLflow promotion -> log summary.

All paths/URLs come from env vars (set in docker-compose.airflow.yml, overridable),
so the same DAG file runs in the container, on WSL, or on bare Linux. Required env
documented in infra/airflow/README.md.
"""
from __future__ import annotations

import os
from datetime import datetime

from airflow import DAG
from airflow.exceptions import AirflowSkipException
from airflow.operators.bash import BashOperator
from airflow.operators.empty import EmptyOperator
from airflow.operators.python import BranchPythonOperator, PythonOperator

REPO = os.environ.get("ZHUCH_REPO_ROOT", "/opt/zhuch")
TRAIN_PY = os.environ.get("ZHUCH_TRAINING_PYTHON", f"{REPO}/training/.venv/bin/python")
ANALYTICS_PY = os.environ.get("ZHUCH_ANALYTICS_PYTHON", f"{REPO}/analytics/.venv/bin/python")
PARQUET_ROOT = os.environ.get("ZHUCH_PARQUET_ROOT", f"{REPO}/analytics/out")
SUMMARY_ROOT = os.environ.get("ZHUCH_SUMMARY_ROOT", f"{REPO}/analytics/out_summary")
RUN_DIR = os.environ.get("ZHUCH_RUN_DIR", f"{REPO}/training/runs/prod")
MODEL_PATH = os.environ.get("ZHUCH_MODEL_PATH", f"{RUN_DIR}/best.json")
ARENA_URL = os.environ.get("ARENA_URL", "http://host.docker.internal:8081")
GATE = os.environ.get("EVAL_GATE_ELO", "1250")


def _branch(ti, dag_run, **_):
    """Retrain when the gate failed, and always on the nightly schedule.
    Manual runs with a passing gate skip straight to the gauntlet."""
    gate = ti.xcom_pull(task_ids="evaluate_current")  # last stdout line: "pass"/"fail"
    if gate == "fail" or dag_run.run_type == "scheduled":
        return "retrain"
    return "skip_retrain"


def _promote(**_):
    uri = os.environ.get("MLFLOW_TRACKING_URI")
    if not uri:
        raise AirflowSkipException("MLFLOW_TRACKING_URI unset — skipping registry promotion")
    import mlflow

    client = mlflow.MlflowClient(tracking_uri=uri)
    versions = client.search_model_versions("name='zhuch-bot'")
    if not versions:
        raise AirflowSkipException("no registered versions of zhuch-bot yet")
    latest = max(versions, key=lambda v: int(v.version))
    client.set_registered_model_alias("zhuch-bot", "production", latest.version)
    print(f"promoted zhuch-bot v{latest.version} -> alias 'production'")


def _notify(ti, **_):
    # ponytail: log-only notify; point this at Slack/email when someone actually
    # needs waking up. states via xcom is enough for the morning glance in the UI.
    gate = ti.xcom_pull(task_ids="evaluate_current")
    print(f"nightly_retrain done: initial gate={gate}, model={MODEL_PATH}, "
          f"summary={SUMMARY_ROOT}/balance.parquet")


with DAG(
    dag_id="nightly_retrain",
    schedule="0 3 * * *",
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["zhuch", "ml"],
) as dag:
    # Placeholder: the sink flushes itself every --flush-interval seconds; when it
    # runs as a long-lived service, replace with a real flush signal (admin endpoint
    # or SIGTERM+restart) so the night's tail of events is on disk before Spark reads.
    sink_flush = BashOperator(
        task_id="sink_flush",
        bash_command='echo "sink_flush: placeholder — sink.py flushes on interval"',
    )

    spark_balance = BashOperator(
        task_id="spark_balance",
        bash_command=(
            f"cd {REPO} && {ANALYTICS_PY} -m analytics.spark_jobs.balance "
            f"--in {PARQUET_ROOT} --out {SUMMARY_ROOT}/balance.parquet"
        ),
    )

    # evaluate.py exits 1 when Elo < gate; convert exit code to a pass/fail string on
    # stdout (BashOperator XComs the last line) so gate failure branches instead of
    # failing the task. A crash (missing model, arena down) also reads as "fail",
    # which for a nightly job degrades to "retrain anyway" — acceptable.
    evaluate_current = BashOperator(
        task_id="evaluate_current",
        bash_command=(
            f"cd {REPO}/training && ({TRAIN_PY} scripts/evaluate.py "
            f"--model {MODEL_PATH} --arena-urls {ARENA_URL} --gate {GATE} "
            f"&& echo pass || echo fail)"
        ),
    )

    branch = BranchPythonOperator(task_id="branch_retrain", python_callable=_branch)

    retrain = BashOperator(
        task_id="retrain",
        bash_command=(
            f"cd {REPO}/training && {TRAIN_PY} scripts/train.py "
            f"--generations 50 --resume {RUN_DIR} --arena-urls {ARENA_URL}"
        ),
    )

    skip_retrain = EmptyOperator(task_id="skip_retrain")

    # Hard gate: exit 1 here fails the task and blocks promotion.
    gauntlet_evaluate = BashOperator(
        task_id="gauntlet_evaluate",
        bash_command=(
            f"cd {REPO}/training && {TRAIN_PY} scripts/evaluate.py "
            f"--model {MODEL_PATH} --arena-urls {ARENA_URL} --gate {GATE}"
        ),
        trigger_rule="none_failed_min_one_success",  # joins the branch
    )

    promote = PythonOperator(task_id="promote", python_callable=_promote)

    notify = PythonOperator(task_id="notify", python_callable=_notify,
                            trigger_rule="all_done")

    sink_flush >> spark_balance >> evaluate_current >> branch
    branch >> [retrain, skip_retrain] >> gauntlet_evaluate >> promote >> notify
