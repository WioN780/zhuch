"""Truncation GA + the shared eval harness (plans, evaluate_population, gauntlet, checkpoints).

es.py reuses everything here except the reproduction step.
"""
from __future__ import annotations

import json
from dataclasses import dataclass, field, asdict
from pathlib import Path

import numpy as np

from .arena_client import ArenaClient
from .fitness import fitness
from .genome import Genome, SIZES
from .league import League, SCRIPTED_SPEC, BASELINE_ELO, elo_update
from .tracking import Tracker


@dataclass
class Config:
    algo: str = "ga"
    generations: int = 100
    pop: int = 256
    sigma: float = 0.03           # ga mutation sigma; es perturbation sigma (default 0.05 via train.py)
    lr: float = 0.01              # es only (Adam)
    elitism: int = 8
    episodes: int = 2             # per-candidate episodes
    max_ticks: int = 1800
    arena_urls: list[str] = field(default_factory=lambda: ["http://127.0.0.1:8081"])
    run_name: str = "run"
    runs_dir: Path = Path("runs")
    seed: int = 0
    gauntlet_every: int = 10
    gauntlet_episodes: int = 50
    hof_margin: float = 25.0
    checkpoint_every: int = 10
    resume: str | None = None

    @property
    def run_dir(self) -> Path:
        return Path(self.runs_dir) / self.run_name


# ---------------- shared eval harness ----------------

def make_plans(n: int, master_seed: int, league: League, cfg: Config) -> list[tuple[list[int], list[dict]]]:
    """Per candidate: (episode seeds, opponent specs). Same opponents across the
    candidate's episodes, different seeds. Reproducible via SeedSequence.spawn."""
    plans = []
    for ss in np.random.SeedSequence(master_seed).spawn(n):
        rng = np.random.default_rng(ss)
        seeds = [int(rng.integers(2 ** 31)) for _ in range(cfg.episodes)]
        opponents = [dict(SCRIPTED_SPEC)] + league.sample(2, rng)
        plans.append((seeds, opponents))
    return plans


def _job(candidate: Genome, opponents: list[dict], seed: int, cfg: Config) -> dict:
    tanks = [{"name": "cand", "weights_b64": candidate.weights_b64()}]
    tanks += [{"name": f"opp{j}", **spec} for j, spec in enumerate(opponents)]
    return {"seed": seed, "max_ticks": cfg.max_ticks, "tanks": tanks}


def evaluate_population(genomes: list[Genome], plans, client: ArenaClient, cfg: Config) -> np.ndarray:
    """Mean fitness over each candidate's episodes. Used by both GA and ES."""
    jobs = [_job(g, opps, s, cfg) for g, (seeds, opps) in zip(genomes, plans) for s in seeds]
    results = client.eval_batch(jobs)
    fits = np.array([fitness({**r["tanks"][0], "ticks": r["ticks"]}, cfg.max_ticks)
                     for r in results], dtype=np.float64)
    return fits.reshape(len(genomes), cfg.episodes).mean(axis=1)


def _episode_score(result: dict) -> float:
    """1v1 win score for tanks[0]: 1 win, 0 loss, 0.5 draw."""
    cand, opp = result["tanks"][0], result["tanks"][1]
    if cand.get("alive") and not opp.get("alive"):
        return 1.0
    if opp.get("alive") and not cand.get("alive"):
        return 0.0
    return 0.5


def gauntlet(genome: Genome, elo: float, league: League, client: ArenaClient,
             cfg: Config, master_seed: int) -> float:
    """~50 1v1 episodes vs scripted + each HOF member, round-robin. Sequential Elo
    updates (K=32); HOF elos move, scripted sentinel stays anchored at 1200.
    Also feeds PFSP winrate tracking."""
    members = league.members()
    seeds = np.random.SeedSequence(master_seed).spawn(cfg.gauntlet_episodes)
    opps, jobs = [], []
    for i, ss in enumerate(seeds):
        m = members[i % len(members)]
        opps.append(m)
        seed = int(np.random.default_rng(ss).integers(2 ** 31))
        jobs.append(_job(genome, [League.spec_for(m)], seed, cfg))
    for m, r in zip(opps, client.eval_batch(jobs)):
        score = _episode_score(r)
        elo, opp_elo = elo_update(elo, m["elo"], score)
        if not m.get("scripted"):
            m["elo"] = opp_elo
        league.record(m, score)
    return elo


# ---------------- checkpointing ----------------

def save_checkpoint(cfg: Config, next_gen: int, rng: np.random.Generator, league: League,
                    best: Genome, best_elo: float, best_fit: float,
                    arrays: dict[str, np.ndarray], extra: dict | None = None) -> None:
    d = cfg.run_dir
    d.mkdir(parents=True, exist_ok=True)
    np.savez(d / "arrays.npz", **arrays)
    league.save(d / "league")
    best.save(d / "best.json")
    state = {"algo": cfg.algo, "gen": next_gen, "best_elo": best_elo, "best_fit": best_fit,
             "rng_state": rng.bit_generator.state, **(extra or {})}
    (d / "state.json").write_text(json.dumps(state))


def load_checkpoint(run_dir: Path) -> tuple[dict, dict, League]:
    state = json.loads((run_dir / "state.json").read_text())
    arrays = dict(np.load(run_dir / "arrays.npz"))
    league = League.load(run_dir / "league")
    return state, arrays, league


def _restore_rng(state: dict) -> np.random.Generator:
    rng = np.random.default_rng()
    rng.bit_generator.state = state["rng_state"]
    return rng


# ---------------- GA ----------------

def run_ga(cfg: Config) -> dict:
    client = ArenaClient(cfg.arena_urls)
    tracker = Tracker(cfg.run_name, asdict(cfg))
    # ponytail: elitism/parents clipped so tiny smoke pops (8) still produce children
    elite_n = min(cfg.elitism, max(1, cfg.pop // 4))
    parent_n = max(1, cfg.pop // 4)

    if cfg.resume:
        state, arrays, league = load_checkpoint(Path(cfg.resume))
        rng = _restore_rng(state)
        population = [Genome(w) for w in arrays["population"]]
        start_gen, best_elo = state["gen"], state["best_elo"]
        best_fit_ever = state["best_fit"]
        best_ever = Genome.load(Path(cfg.resume) / "best.json")
    else:
        rng = np.random.default_rng(cfg.seed)
        league = League()
        population = [Genome.he_init(rng) for _ in range(cfg.pop)]
        start_gen, best_elo, best_fit_ever = 0, BASELINE_ELO, -np.inf
        best_ever = population[0]

    for gen in range(start_gen, cfg.generations):
        master = int(rng.integers(2 ** 63))
        fits = evaluate_population(population, make_plans(cfg.pop, master, league, cfg), client, cfg)
        order = np.argsort(-fits)
        gen_best = population[int(order[0])]
        if fits[order[0]] > best_fit_ever:
            best_fit_ever, best_ever = float(fits[order[0]]), gen_best
        metrics = {"fit_mean": float(fits.mean()), "fit_max": float(fits.max()),
                   "fit_best_ever": best_fit_ever, "elo_best": best_elo}
        tracker.log_metrics(metrics, step=gen)
        print(f"[ga gen {gen}] fit mean={metrics['fit_mean']:.1f} max={metrics['fit_max']:.1f} "
              f"best_ever={best_fit_ever:.1f} elo={best_elo:.0f}", flush=True)

        if (gen + 1) % cfg.gauntlet_every == 0:
            best_elo = gauntlet(gen_best, best_elo, league, client, cfg, int(rng.integers(2 ** 63)))
            if best_elo > league.max_elo() + cfg.hof_margin:
                league.snapshot(gen_best, best_elo, gen)
            tracker.log_metrics({"elo_best": best_elo}, step=gen)

        elites = [population[int(i)] for i in order[:elite_n]]
        parents = [population[int(i)] for i in order[:parent_n]]
        children = [parents[int(rng.integers(len(parents)))].gaussian_mutate(rng, cfg.sigma)
                    for _ in range(cfg.pop - elite_n)]
        population = elites + children

        if (gen + 1) % cfg.checkpoint_every == 0 or gen == cfg.generations - 1:
            save_checkpoint(cfg, gen + 1, rng, league, best_ever, best_elo, best_fit_ever,
                            {"population": np.stack([g.weights for g in population])})
            tracker.log_artifact(cfg.run_dir / "best.json")

    tracker.close()
    return {"best_fitness": best_fit_ever, "best_elo": best_elo, "run_dir": cfg.run_dir}
