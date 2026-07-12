"""Genome: flat float32 weight vector + model JSON format per docs/contracts.md section 3."""
from __future__ import annotations

import base64
import hashlib
import json
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np

SIZES: tuple[int, ...] = (90, 64, 64, 5)


def param_count(sizes: tuple[int, ...] = SIZES) -> int:
    return sum(sizes[i + 1] * sizes[i] + sizes[i + 1] for i in range(len(sizes) - 1))


@dataclass
class Genome:
    weights: np.ndarray  # flat float32, layer-major: W(out,in) row-major then bias
    sizes: tuple[int, ...] = SIZES

    def __post_init__(self) -> None:
        self.weights = np.asarray(self.weights, dtype="<f4")
        self.sizes = tuple(self.sizes)
        if self.weights.size != param_count(self.sizes):
            raise ValueError(
                f"weights len {self.weights.size} != param_count {param_count(self.sizes)}"
            )

    @classmethod
    def he_init(cls, rng: np.random.Generator, sizes: tuple[int, ...] = SIZES) -> "Genome":
        chunks = []
        for i in range(len(sizes) - 1):
            nin, nout = sizes[i], sizes[i + 1]
            chunks.append(rng.standard_normal(nout * nin) * np.sqrt(2.0 / nin))
            chunks.append(np.zeros(nout))
        return cls(np.concatenate(chunks).astype("<f4"), sizes)

    def gaussian_mutate(self, rng: np.random.Generator, sigma: float) -> "Genome":
        w = self.weights.astype(np.float64) + rng.standard_normal(self.weights.size) * sigma
        return Genome(w.astype("<f4"), self.sizes)

    def raw_bytes(self) -> bytes:
        return self.weights.astype("<f4").tobytes()

    def weights_b64(self) -> str:
        return base64.b64encode(self.raw_bytes()).decode()

    def sha256(self) -> str:
        return hashlib.sha256(self.raw_bytes()).hexdigest()

    def to_model_json(self) -> dict:
        return {
            "format": "zhuch-mlp",
            "version": 1,
            "sizes": list(self.sizes),
            "sha256": self.sha256(),
            "weights_b64": self.weights_b64(),
        }

    @classmethod
    def from_model_json(cls, doc: dict) -> "Genome":
        if doc.get("format") != "zhuch-mlp" or doc.get("version") != 1:
            raise ValueError(f"unsupported model format: {doc.get('format')} v{doc.get('version')}")
        raw = base64.b64decode(doc["weights_b64"])
        if hashlib.sha256(raw).hexdigest() != doc["sha256"]:
            raise ValueError("sha256 mismatch: corrupt weights")
        return cls(np.frombuffer(raw, dtype="<f4").copy(), tuple(doc["sizes"]))

    def save(self, path: Path | str) -> None:
        Path(path).write_text(json.dumps(self.to_model_json()))

    @classmethod
    def load(cls, path: Path | str) -> "Genome":
        return cls.from_model_json(json.loads(Path(path).read_text()))
