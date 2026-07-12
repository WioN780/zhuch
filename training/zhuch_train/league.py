"""Hall-of-fame league with PFSP-lite opponent sampling and Elo bookkeeping."""
from __future__ import annotations

import json
from pathlib import Path

import numpy as np

from .genome import Genome

SCRIPTED_SPEC = {"scripted": True}
BASELINE_ELO = 1200.0


def elo_update(ra: float, rb: float, score_a: float, k: float = 32.0) -> tuple[float, float]:
    """Standard Elo. score_a: 1 win, 0.5 draw, 0 loss for player a."""
    ea = 1.0 / (1.0 + 10.0 ** ((rb - ra) / 400.0))
    return ra + k * (score_a - ea), rb + k * ((1.0 - score_a) - (1.0 - ea))


class League:
    """HOF cap 20 + always-available scripted sentinel.

    Each member tracks (wins, games) vs the current best candidate; PFSP-lite
    samples opponents with P proportional to wr*(1-wr), wr laplace-smoothed.
    """

    def __init__(self, cap: int = 20):
        self.cap = cap
        # sentinel elo is anchored at BASELINE_ELO and never updated
        self.sentinel: dict = {"scripted": True, "elo": BASELINE_ELO, "wins": 0, "games": 0}
        self.hof: list[dict] = []  # {"genome": Genome, "elo", "gen", "wins", "games"}

    def members(self) -> list[dict]:
        return [self.sentinel] + self.hof

    def max_elo(self) -> float:
        return max((m["elo"] for m in self.hof), default=BASELINE_ELO)

    def snapshot(self, genome: Genome, elo: float, gen: int) -> None:
        self.hof.append({"genome": genome, "elo": float(elo), "gen": int(gen),
                         "wins": 0, "games": 0})
        if len(self.hof) > self.cap:
            self.hof.sort(key=lambda m: m["elo"], reverse=True)
            self.hof = self.hof[:self.cap]

    @staticmethod
    def spec_for(member: dict) -> dict:
        if member.get("scripted"):
            return dict(SCRIPTED_SPEC)
        return {"weights_b64": member["genome"].weights_b64()}

    def record(self, member: dict, score: float) -> None:
        member["games"] += 1
        member["wins"] += score  # draws count 0.5

    def sample(self, k: int, rng: np.random.Generator) -> list[dict]:
        """k opponent tank-specs, PFSP-lite over sentinel + HOF."""
        pool = self.members()
        wr = np.array([(m["wins"] + 1.0) / (m["games"] + 2.0) for m in pool])
        w = wr * (1.0 - wr)
        p = w / w.sum()
        idx = rng.choice(len(pool), size=k, replace=True, p=p)
        return [self.spec_for(pool[i]) for i in idx]

    # -- persistence: a directory of genome jsons + league.json meta --

    def save(self, dir: Path | str) -> None:
        d = Path(dir)
        d.mkdir(parents=True, exist_ok=True)
        meta = {"cap": self.cap,
                "sentinel": {k: self.sentinel[k] for k in ("elo", "wins", "games")},
                "members": []}
        for i, m in enumerate(self.hof):
            fname = f"hof_{i:02d}.json"
            m["genome"].save(d / fname)
            meta["members"].append({"file": fname, "elo": m["elo"], "gen": m["gen"],
                                    "wins": m["wins"], "games": m["games"]})
        (d / "league.json").write_text(json.dumps(meta, indent=1))

    @classmethod
    def load(cls, dir: Path | str) -> "League":
        d = Path(dir)
        meta = json.loads((d / "league.json").read_text())
        lg = cls(cap=meta["cap"])
        lg.sentinel.update(meta["sentinel"])
        for m in meta["members"]:
            lg.hof.append({"genome": Genome.load(d / m["file"]), "elo": m["elo"],
                           "gen": m["gen"], "wins": m["wins"], "games": m["games"]})
        return lg
