"""HTTP client for the Go arena (contracts section 4). Works against scripts/mock_arena.py too."""
from __future__ import annotations

import time
from concurrent.futures import ThreadPoolExecutor

import numpy as np
import requests


class ArenaClient:
    def __init__(self, urls: list[str], timeout: float = 600.0,
                 retries: int = 3, backoff: float = 1.0):
        if not urls:
            raise ValueError("need at least one arena URL")
        self.urls = [u.rstrip("/") for u in urls]
        self.timeout = timeout
        self.retries = retries
        self.backoff = backoff

    def _post(self, url: str, path: str, payload: dict) -> dict:
        last: Exception | None = None
        for attempt in range(self.retries + 1):
            try:
                r = requests.post(url + path, json=payload, timeout=self.timeout)
                if r.status_code >= 500:
                    raise RuntimeError(f"{url}{path} -> HTTP {r.status_code}")
                r.raise_for_status()
                return r.json()
            except (requests.RequestException, RuntimeError) as e:
                last = e
                if attempt < self.retries:
                    time.sleep(self.backoff * 2 ** attempt)
        raise RuntimeError(f"arena request failed after {self.retries + 1} attempts: {last}")

    def eval_batch(self, jobs: list[dict], max_chunk_size: int = 128) -> list[dict]:
        """POST jobs split across arena URLs, chunked to prevent OOM errors on the server."""
        if not jobs:
            return []
        chunks = [jobs[i:i + max_chunk_size] for i in range(0, len(jobs), max_chunk_size)]
        results: list[dict | None] = [None] * len(jobs)

        def post_chunk_with_offset(url: str, chunk: list[dict], offset: int) -> None:
            resp = self._post(url, "/eval_batch", {"jobs": chunk})
            results[offset:offset + len(chunk)] = resp["results"]

        with ThreadPoolExecutor(max_workers=len(self.urls)) as ex:
            futs = []
            for idx, chunk in enumerate(chunks):
                url = self.urls[idx % len(self.urls)]
                offset = idx * max_chunk_size
                futs.append(ex.submit(post_chunk_with_offset, url, chunk, offset))
            for f in futs:
                f.result()  # re-raise failures
        return results  # type: ignore[return-value]

    def forward(self, sizes: list[int], weights_b64: str, input: list[float]) -> list[float]:
        resp = self._post(self.urls[0], "/forward",
                          {"sizes": sizes, "weights_b64": weights_b64, "input": input})
        return resp["output"]
