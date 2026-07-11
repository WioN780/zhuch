# zhuch MLOps

Local stack for experiment tracking (MLflow), metrics (Prometheus), and dashboards (Grafana). Everything runs in Docker; training and the Go binaries run on the host.

## Start the stack

```bash
cd infra/compose
docker compose up -d --build   # --build: mlflow image adds psycopg2-binary + boto3 on top of the official image
```

Optional: `cp .env.example .env` to change creds (defaults are minioadmin/minioadmin, mlflow/mlflow — local only).

| Service | URL |
|---|---|
| MLflow | http://localhost:5000 |
| Grafana | http://localhost:3000 (anonymous admin, dashboards auto-provisioned) |
| Prometheus | http://localhost:9090 |
| MinIO console | http://localhost:9001 |

MLflow uses postgres as backend store and MinIO bucket `mlflow` as artifact store (server-proxied, so training on the host needs no S3 creds — only the tracking URI).

## Training side

```bash
export MLFLOW_TRACKING_URI=http://localhost:5000
```

The training Tracker logs per run:

- **Params**: population size, generations, mutation sigma, fitness weights (`w_score`, `w_kill`, `w_surv` — contracts §7), arena URL, seed.
- **Per-generation metrics**: `fit_mean`, `fit_max`, `elo_best`.
- **Artifacts**: best genome per generation as a `zhuch-mlp` v1 JSON weight file (contracts §3).

Prometheus scrapes the host-side Go processes via `host.docker.internal`: game server `:8080/metrics`, arena `:8081/metrics` (metric names pinned in contracts §8). Grafana ships two dashboards: **Live Game** (tick p50/p99 vs 50ms budget, entities/players/bots per room, room count) and **Training Throughput** (episodes/s, ticks/s, episode duration p50/p95, in-flight evals).

## Model registry / promotion

1. Training registers candidate genomes as versions of registered model `zhuch-bot`.
2. `evaluate.py` runs the gauntlet: candidate vs the frozen scripted baseline (contracts §9) and current production, via arena `/eval_batch`.
3. Elo gate: candidate must beat the current `production` alias by the configured Elo margin.
4. On pass, promote: set the `production` alias on the winning version (`mlflow.MlflowClient().set_registered_model_alias("zhuch-bot", "production", version)`).
5. Game server deploy pulls the weight-file artifact for `zhuch-bot@production` and loads it via `backend/pkg/brain`.

No automation around this yet — the gate is a script you run; wire it into CI when the loop is stable.

## Later: Cloudflare R2 instead of MinIO

MLflow talks generic S3, so swapping the artifact store is config, not code: point `MLFLOW_S3_ENDPOINT_URL` at the R2 endpoint (`https://<account_id>.r2.cloudflarestorage.com`), set R2 access keys as `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`, keep `--artifacts-destination s3://mlflow`. A Terraform workstream provisions the R2 bucket + keys; until then MinIO is a drop-in local stand-in.
