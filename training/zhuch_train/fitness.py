"""Fitness composition per docs/contracts.md section 7 (v2: accuracy terms)."""


def fitness(stats: dict, max_ticks: int,
            w_score: float = 1.0, w_kill: float = 200.0, w_surv: float = 100.0,
            w_hit: float = 10.0, w_miss: float = -2.0) -> float:
    """stats: eval-response tank entry merged with the episode's top-level "ticks".

    w_miss is deliberately gentle (small negative) so it doesn't suppress fire
    exploration early in training; raise its magnitude if spray-and-pray
    persists once candidates are landing hits reliably.
    """
    death_tick = stats.get("death_tick", -1)
    survived = death_tick if death_tick >= 0 else stats.get("ticks", max_ticks)
    shots_fired = stats.get("shots_fired", 0)
    hits_tank = stats.get("hits_tank", 0)
    hits_food = stats.get("hits_food", 0)
    misses = shots_fired - hits_tank - hits_food
    return (w_score * stats.get("score", 0)
            + w_kill * stats.get("kills", 0)
            + w_surv * (survived / max_ticks)
            + w_hit * hits_tank
            + w_miss * misses)
