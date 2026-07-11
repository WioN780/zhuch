"""Fitness composition per docs/contracts.md section 7."""


def fitness(stats: dict, max_ticks: int,
            w_score: float = 1.0, w_kill: float = 200.0, w_surv: float = 100.0) -> float:
    """stats: eval-response tank entry merged with the episode's top-level "ticks"."""
    death_tick = stats.get("death_tick", -1)
    survived = death_tick if death_tick >= 0 else stats.get("ticks", max_ticks)
    return (w_score * stats.get("score", 0)
            + w_kill * stats.get("kills", 0)
            + w_surv * (survived / max_ticks))
