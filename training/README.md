# zhuch-train

Neuroevolution training (GA + OpenAI-ES) for zhuch tank bots. Evaluates candidates
against the arena HTTP service (`backend/cmd/arena`, contracts in `docs/contracts.md`).

## Setup (Windows)

```powershell
cd training
python -m venv .venv
.venv\Scripts\pip install -e .[dev]
```

## Test

```powershell
.venv\Scripts\pytest
```

The real-arena parity test skips unless `ARENA_URL` is set (e.g. `http://127.0.0.1:8081`).

## Train

Against the real arena (or the offline mock: `python scripts/mock_arena.py --port 8081`):

```powershell
.venv\Scripts\python scripts\train.py --algo ga --generations 100 --arena-urls http://127.0.0.1:8081
.venv\Scripts\python scripts\train.py --algo es --generations 100 --pop 256 --sigma 0.05
```

Checkpoints (population/theta + league + rng state) land in `runs/<name>/` every 10
generations; resume with `--resume runs/<name>`. Set `MLFLOW_TRACKING_URI` (and
`pip install -e .[tracking]`) for MLflow logging; otherwise tracking is a no-op.

## Evaluate / promotion gate

```powershell
.venv\Scripts\python scripts\evaluate.py --model runs/<name>/best.json --arena-urls http://127.0.0.1:8081 --gate 1250
```

Runs a 50-episode gauntlet vs the scripted baseline, reports W/L/D, mean fitness and
Elo vs baseline-1200. Exit code 0 iff Elo >= gate.
