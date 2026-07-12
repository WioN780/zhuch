"""Promote a trained genome to production.

Runs the evaluate.py gauntlet (Elo gate vs the frozen scripted baseline),
and only on a pass: copies the genome to backend/models/champion.json and,
when MLFLOW_TRACKING_URI is set, registers it in the MLflow model registry
under the `champion` alias.

    python scripts/promote.py --model runs/<name>/best.json \
        --arena-urls http://127.0.0.1:8081 --gate 1250

After promoting, redeploy the game server (Railway rebuilds the image, which
bakes in backend/models/champion.json) — see docs/next-steps.md.
"""
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path

# mlflow prints emoji on end_run; on Windows cp1252 consoles the
# UnicodeEncodeError would abort the registry steps (same fix as tracking.py).
for _s in (sys.stdout, sys.stderr):
    if hasattr(_s, "reconfigure"):
        _s.reconfigure(errors="replace")

REPO_ROOT = Path(__file__).resolve().parents[2]
CHAMPION = REPO_ROOT / "backend" / "models" / "champion.json"


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--model", required=True, help="genome JSON (e.g. runs/<name>/best.json)")
    ap.add_argument("--arena-urls", nargs="+", default=["http://127.0.0.1:8081"])
    ap.add_argument("--gate", type=float, default=1250.0)
    ap.add_argument("--episodes", type=int, default=50)
    args = ap.parse_args()

    # Reuse evaluate.py verbatim as the gate: exit code 0 iff elo >= gate.
    gauntlet = subprocess.run(
        [sys.executable, str(Path(__file__).parent / "evaluate.py"),
         "--model", args.model, "--gate", str(args.gate),
         "--episodes", str(args.episodes), "--arena-urls", *args.arena_urls],
    )
    if gauntlet.returncode != 0:
        print(f"GATE FAILED (elo < {args.gate}): not promoting")
        sys.exit(1)

    CHAMPION.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(args.model, CHAMPION)
    print(f"promoted -> {CHAMPION}")

    if os.environ.get("MLFLOW_TRACKING_URI"):
        try:
            import mlflow
            from mlflow import MlflowClient
            mlflow.set_tracking_uri(os.environ["MLFLOW_TRACKING_URI"])
            client = MlflowClient()
            name = "zhuch-bot"
            try:
                client.create_registered_model(name)
            except Exception:
                pass  # already exists
            with mlflow.start_run(run_name="promotion") as run:
                mlflow.log_artifact(args.model)
                mv = client.create_model_version(
                    name=name, source=run.info.artifact_uri, run_id=run.info.run_id)
            client.set_registered_model_alias(name, "champion", mv.version)
            print(f"mlflow registry: {name} v{mv.version} aliased 'champion'")
        except Exception as e:
            print(f"mlflow registry step failed (promotion still done): {e}")

    print("next: commit backend/models/champion.json and redeploy (docs/next-steps.md)")


if __name__ == "__main__":
    main()
