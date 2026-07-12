# zhuch Airflow (nightly retrain)

Single-node Airflow (LocalExecutor, `apache/airflow:2.10.5-python3.11`) running
`dags/nightly_retrain.py`: nightly at 03:00 — Spark balance report -> Elo gate on the
current model -> retrain (`--generations 50 --resume`) when the gate fails or on any
scheduled run -> gauntlet re-evaluation (hard gate) -> MLflow registry promotion
(`zhuch-bot` @ `production`) -> log summary.

## Run

```bash
cd infra/airflow
docker compose -f docker-compose.airflow.yml up airflow-init   # one-shot: db + admin user
docker compose -f docker-compose.airflow.yml up -d
```

UI: http://localhost:8090 (admin/admin). The DAG starts paused — unpause it in the UI.

Linux containers only, so on Windows run this from WSL (Docker Desktop works either
way, but the mounted repo's venvs must be *Linux* venvs — build
`training/.venv` and `analytics/.venv` from WSL if the containers will call them).

## Configuration (env vars)

Everything the DAG shells out to is an env var with a container-friendly default, set
in `docker-compose.airflow.yml` and overridable via the host environment / an `.env`
file next to the compose file. No Airflow Variables are required — env only, so the
DAG file also runs under a bare `airflow standalone` on WSL/Linux.

| Env var | Default | Meaning |
|---|---|---|
| `ZHUCH_REPO_ROOT` | `/opt/zhuch` (host: `../..`) | Repo checkout; bind-mounted into the containers |
| `ZHUCH_TRAINING_PYTHON` | `$REPO/training/.venv/bin/python` | Python with `zhuch-train` installed |
| `ZHUCH_ANALYTICS_PYTHON` | `$REPO/analytics/.venv/bin/python` | Python with `zhuch-analytics[spark]` installed |
| `ZHUCH_PARQUET_ROOT` | `$REPO/analytics/out` | Sink output tree (Spark input) |
| `ZHUCH_SUMMARY_ROOT` | `$REPO/analytics/out_summary` | Balance report output |
| `ZHUCH_RUN_DIR` | `$REPO/training/runs/prod` | Checkpoint dir passed to `train.py --resume` |
| `ZHUCH_MODEL_PATH` | `$ZHUCH_RUN_DIR/best.json` | Model evaluated/gauntleted |
| `ARENA_URL` | `http://host.docker.internal:8081` | Go arena (or `training/scripts/mock_arena.py`) |
| `MLFLOW_TRACKING_URI` | *(unset)* | Unset -> `promote` task skips cleanly |
| `EVAL_GATE_ELO` | `1250` | Elo threshold for both gate tasks |

## Task graph

```
sink_flush -> spark_balance -> evaluate_current -> branch_retrain
                                          |-> retrain      -\
                                          |-> skip_retrain --+-> gauntlet_evaluate -> promote -> notify
```

- `sink_flush` is a placeholder (`analytics/sink.py` flushes on `--flush-interval`);
  replace with a real flush signal once the sink runs as a long-lived service.
- `evaluate_current` never fails the run: gate failure just steers the branch.
  `gauntlet_evaluate` is the hard gate — its failure blocks `promote`.
- `promote` sets the `production` alias on the newest registered `zhuch-bot` version
  (docs/mlops.md); it skips (not fails) when `MLFLOW_TRACKING_URI` is unset.
