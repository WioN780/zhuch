"""MLflow tracking wrapper: real logging iff MLFLOW_TRACKING_URI is set and mlflow imports.

Never crashes training — every mlflow call is wrapped.
"""
from __future__ import annotations

import os


class Tracker:
    def __init__(self, run_name: str, params: dict):
        self._mlflow = None
        uri = os.environ.get("MLFLOW_TRACKING_URI")
        if not uri:
            return
        try:
            import sys
            # mlflow prints emoji (🏃) on end_run; on Windows cp1252 consoles
            # the resulting UnicodeEncodeError aborts set_terminated and every
            # run is left in RUNNING state forever.
            for stream in (sys.stdout, sys.stderr):
                if hasattr(stream, "reconfigure"):
                    stream.reconfigure(errors="replace")
            import mlflow
            mlflow.set_tracking_uri(uri)
            mlflow.set_experiment("zhuch")
            mlflow.start_run(run_name=run_name)
            mlflow.log_params({k: str(v) for k, v in params.items()})
            self._mlflow = mlflow
        except Exception:
            self._mlflow = None

    def log_metrics(self, metrics: dict, step: int) -> None:
        if self._mlflow:
            try:
                self._mlflow.log_metrics({k: float(v) for k, v in metrics.items()}, step=step)
            except Exception:
                pass

    def log_artifact(self, path) -> None:
        if self._mlflow:
            try:
                self._mlflow.log_artifact(str(path))
            except Exception:
                pass

    def close(self) -> None:
        if self._mlflow:
            try:
                self._mlflow.end_run()
            except Exception:
                pass
