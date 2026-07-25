# Zhuch

A multiplayer [Diep.io](https://diep.io)-style tank arena where the bots are neural
networks, bred through evolutionary algorithms and evaluated in a
headless Go arena, with a full MLOps loop (experiment tracking, telemetry, and
promotion gates) behind them.

**Live demo:** https://zhuch.markooba.com

[![CI](https://github.com/WioN780/zhuch/actions/workflows/ci.yml/badge.svg)](https://github.com/WioN780/zhuch/actions/workflows/ci.yml)

---

## What's here

- A real-time multiplayer game server and WebGL client, playable on its own.
- Bots driven by small MLPs (90 in → 64 → 64 → 5 out) instead of hand-tuned
  heuristics, trained with a Genetic Algorithm and OpenAI-ES against a
  deterministic, headless physics arena.
- A promotion pipeline: candidates are gauntlet-tested against the reigning
  champion and against a frozen scripted baseline, gated on Elo, before they
  replace the bots players actually see.
- A telemetry/MLOps stack (Kafka, MLflow, Prometheus/Grafana, Spark) so
  training runs and live matches are both observable, not just runnable.

## Tech stack

| Layer | Tech |
|---|---|
| Game server & headless arena | Go, [gorilla/websocket](https://github.com/gorilla/websocket), custom 2D physics engine, [franz-go](https://github.com/twmb/franz-go) (Kafka), [Prometheus client](https://github.com/prometheus/client_golang) |
| Game client | JavaScript, [Pixi.js](https://pixijs.com/) (WebGL), Vite |
| Bot training | Python, NumPy, Genetic Algorithm + OpenAI-ES neuroevolution, MLflow tracking |
| Telemetry & analytics | Kafka (Redpanda), Spark, Parquet, Prometheus, Grafana |
| Model/artifact storage | MinIO (S3-compatible), Postgres (MLflow backend store) |
| Infra as code | Docker Compose (local stack), Helm (k8s scale-out), Terraform (Railway + Cloudflare R2) |
| CI/CD | GitHub Actions (Go + Python + Go↔Python parity + frontend build), GitLab mirror pipeline |
| Hosting (demo) | [Railway](https://railway.app) |

## Repository layout

- **[`backend/`](backend)**: Go.
  - `cmd/server`: the real-time WebSocket room coordinator (port 8080) that
    serves actual multiplayer games.
  - `cmd/arena`: a headless, fast-forward simulator (port 8081) used for
    training evaluation and Go↔Python physics/observation parity checks.
  - `pkg/engine`: deterministic 2D physics: tanks, bullets, food, obstacles,
    collisions.
  - `pkg/bots`: builds the observation vector fed to trained models and
    applies their output; falls back to a frozen scripted policy if no
    trained model is present.
  - `models/`: the bot weight files the server loads (see
    [`backend/models/README.md`](backend/models/README.md) for the format).
- **[`frontend/`](frontend)**: the WebGL game client (Pixi.js + Vite, port 5173).
- **[`training/`](training)**: the Python neuroevolution framework
  (`zhuch_train/`: genome format, GA/ES, fitness, arena client) plus
  `scripts/train.py` and `scripts/promote.py`.
- **[`analytics/`](analytics)**: Kafka → Parquet → Spark pipeline for match
  telemetry.
- **[`infra/`](infra)**: `compose/` (local Postgres/MinIO/MLflow/
  Prometheus/Grafana/Redpanda stack), `helm/` (k8s scale-out chart),
  `terraform/` (Railway + R2), `airflow/` (scheduled analytics DAGs).

Internal working docs (contracts, runbooks, blog drafts) are kept out of the
public repo. See [`docs/README.md`](docs/README.md) for why, and for pointers
to the architecture docs that *are* public (the `README.md` files linked
above).

## Local development quickstart

### 1. Telemetry & MLOps stack (optional for just playing the game)
```powershell
cd infra/compose
docker compose up -d
```
- MLflow (experiment/model registry): http://localhost:5000
- Grafana (live metrics): http://localhost:3000
- MinIO console (artifact storage): http://localhost:9001

### 2. Go backends
```powershell
cd backend
$env:KAFKA_BROKERS="localhost:19092"   # omit to run without telemetry
go run ./cmd/server
```
In another terminal, the headless arena (only needed for training/parity):
```powershell
cd backend
go run ./cmd/arena
```

### 3. Game client
```powershell
cd frontend
npm install
npm run dev
```
Open http://localhost:5173, pick "Local Server", enter a nickname and a game
mode, and play against the bots. `VITE_BACKEND_URL` (see below) controls
which server the built client points at by default.

## Training and promoting bots

Bots observe a 90-float vector (own state, nearby tanks/bullets/food, and
static obstacles) and output 5 floats (steering, orientation, firing).

```powershell
cd training
python -m venv .venv
.venv\Scripts\activate
pip install -e .[dev]

# Train (Genetic Algorithm or OpenAI-ES), tracked in MLflow:
$env:MLFLOW_TRACKING_URI="http://localhost:5000"
.venv\Scripts\python scripts\train.py --algo ga --pop 512 --generations 2000 `
  --arena-urls http://127.0.0.1:8081 --run-name ga-v2
# --resume runs/ga-v2 to continue a paused/crashed run

# Evaluate a candidate in a 50-episode gauntlet vs. the scripted baseline;
# if it clears the Elo gate it's registered in MLflow and promoted:
.venv\Scripts\python scripts\promote.py --model runs/ga-v2/best.json `
  --arena-urls http://127.0.0.1:8081 --gate 1350
```
Promotion writes `backend/models/champion.json`, which the game server loads
at startup (and falls back off of, to a scripted bot, if it's missing or
fails to validate).

## Testing

- Go engine + observation tests: `go test ./...` in [`backend/`](backend)
- Python training tests: `pytest` in [`training/`](training) (`.venv\Scripts\pytest`)
- Go↔Python physics/observation parity + a live `/eval` smoke test: see the
  `integration` job in [`.github/workflows/ci.yml`](.github/workflows/ci.yml)
- Frontend: `npm run build` in [`frontend/`](frontend)

All of the above run on every push/PR via GitHub Actions (also mirrored to a
GitLab CI pipeline for redundancy).

## Deployment

The demo at https://zhuch.markooba.com runs two Railway services built from
this repo:

- **backend**: `backend/Dockerfile`, the WebSocket game server (bundles
  `backend/models/champion.json` so bots work with no extra setup).
- **frontend**: static Vite build; `VITE_BACKEND_URL` (a Railway service
  variable, e.g. `backend-production-xxxx.up.railway.app`) is baked into the
  bundle at build time so the client knows which backend to talk to. Without
  it, the built client defaults to `localhost:8080`.

`infra/terraform/railway.tf` documents (and can adopt-as-code) the Railway
project/service/variable config; `infra/helm/` has a Kubernetes chart for
scaling the arena horizontally if/when training moves off a single host.
