"""Synthetic telemetry event generator for zhuch.events.v1 (docs/contracts.md §6).

Produces plausible episodes for three game modes (zombie / boss / ffa) without the
real Go engine: spawn -> periodic pos_sample (1Hz) + damage/kill/death -> episode_end.
Lets the Kafka -> Parquet -> Spark pipeline be developed and tested standalone before
the Go producers are wired up (they gate emission on KAFKA_BROKERS, same as here).

CLI:
    python -m analytics.synthetic --episodes 200 --out events.ndjson
    python -m analytics.synthetic --episodes 200 --brokers localhost:9092
"""
from __future__ import annotations

import argparse
import json
import sys
import time
import uuid
from typing import Iterator, Optional

import numpy as np

TOPIC = "zhuch.events.v1"
MODES = ("zombie", "boss", "ffa")
MAP_SIZE = 2000.0
TICK_HZ = 20  # backend engine tick rate (contracts §2)
TICK_MS = 1000 // TICK_HZ
POS_SAMPLE_TICKS = 20  # 1Hz (contracts §6)

# Per-mode bot-survival bias so downstream balance/difficulty jobs have real signal
# to react to: zombies (bots) tend to win zombie mode, humans tend to win boss mode.
_MODE_BOT_SURVIVE_BIAS = {"zombie": 0.65, "boss": 0.35, "ffa": 0.50}


def _rand_pos(rng: np.random.Generator) -> dict:
    return {"x": float(rng.uniform(0, MAP_SIZE)), "y": float(rng.uniform(0, MAP_SIZE))}


def _walk_pos(rng: np.random.Generator, pos: dict, step: float = 60.0) -> dict:
    return {
        "x": float(min(max(pos["x"] + rng.normal(0, step), 0.0), MAP_SIZE)),
        "y": float(min(max(pos["y"] + rng.normal(0, step), 0.0), MAP_SIZE)),
    }


def _tank_ids(rng: np.random.Generator) -> list[str]:
    n_bots = int(rng.integers(2, 7))
    n_humans = 0 if rng.random() < 0.3 else int(rng.integers(1, 3))
    ids = [f"bot-{i}" for i in range(n_bots)] + [f"human-{i}" for i in range(n_humans)]
    rng.shuffle(ids)
    return ids


def generate_episode(rng: np.random.Generator, room: str, ep_start_ms: int) -> Iterator[dict]:
    """Yield one full episode's events (spawn..episode_end), ts = ep_start_ms + tick*TICK_MS."""
    mode = str(rng.choice(MODES))
    episode_id = str(uuid.UUID(bytes=rng.bytes(16)))  # rng-derived: reproducible per seed
    tanks = _tank_ids(rng)
    ticks_total = int(rng.integers(300, 1800))

    def emit(tick: int, type_: str, actor, target=None, pos=None, extra: Optional[dict] = None) -> dict:
        data = {"mode": mode}
        if extra:
            data.update(extra)
        return {
            "ts": ep_start_ms + tick * TICK_MS, "source": "arena", "room": room,
            "episode": episode_id, "type": type_, "actor": actor, "target": target,
            "pos": pos, "data": data,
        }

    pos = {t: _rand_pos(rng) for t in tanks}
    for t in tanks:
        yield emit(0, "spawn", t, None, pos[t])

    survive_bias = _MODE_BOT_SURVIVE_BIAS[mode]
    death_tick = {}
    for t in tanks:
        survive_p = survive_bias if t.startswith("bot") else (1 - survive_bias)
        death_tick[t] = int(rng.integers(20, ticks_total)) if rng.random() > survive_p else -1

    kills = {t: 0 for t in tanks}
    score = {t: 0 for t in tanks}
    living = set(tanks)

    for tick in range(0, ticks_total + 1, POS_SAMPLE_TICKS):
        for t in tanks:
            if t not in living:
                continue
            pos[t] = _walk_pos(rng, pos[t])
            yield emit(tick, "pos_sample", t, None, pos[t])

        for t in list(living):
            if death_tick[t] != -1 and death_tick[t] <= tick:
                others = [o for o in tanks if o != t] or [t]
                attacker = str(rng.choice(others))
                yield emit(death_tick[t], "damage", attacker, t, pos[t],
                            {"amount": round(float(rng.uniform(10, 40)), 1)})
                yield emit(death_tick[t], "kill", attacker, t, pos[t])
                yield emit(death_tick[t], "death", t, attacker, pos[t])
                kills[attacker] += 1
                score[attacker] += int(rng.integers(20, 60))
                living.discard(t)

        if rng.random() < 0.3 and len(living) >= 2:
            a, b = rng.choice(sorted(living), size=2, replace=False)
            yield emit(tick, "damage", str(a), str(b), pos[str(b)],
                        {"amount": round(float(rng.uniform(1, 15)), 1)})
            score[str(a)] += 1

    for t in tanks:
        score[t] += int(rng.integers(0, 50))
    tanks_summary = [
        {"name": t, "score": score[t], "kills": kills[t],
         "death_tick": death_tick[t], "alive": t in living}
        for t in tanks
    ]
    yield emit(ticks_total, "episode_end", "arena", None, None,
                {"ticks": ticks_total, "tanks": tanks_summary})


def iter_events(n_episodes: int, seed: Optional[int] = None,
                 spread_seconds: float = 3 * 3600.0) -> Iterator[dict]:
    """Iterator API: n_episodes worth of events, start times spread across
    spread_seconds so a downstream hourly-partitioned sink sees multiple partitions."""
    rng = np.random.default_rng(seed)
    base_ms = int(time.time() * 1000) - int(spread_seconds * 1000)
    for i in range(n_episodes):
        room = f"synthetic-room-{i % 8}"
        ep_start = base_ms + int((i / max(n_episodes - 1, 1)) * spread_seconds * 1000)
        yield from generate_episode(rng, room, ep_start)


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                  formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--episodes", type=int, default=200)
    ap.add_argument("--seed", type=int, default=None)
    ap.add_argument("--out", help="NDJSON output path ('-' for stdout)")
    ap.add_argument("--brokers", nargs="+", help="Kafka bootstrap servers, e.g. localhost:9092")
    ap.add_argument("--topic", default=TOPIC)
    ap.add_argument("--spread-hours", type=float, default=3.0,
                     help="spread episode start times across N hours")
    args = ap.parse_args(argv)
    if bool(args.out) == bool(args.brokers):
        ap.error("specify exactly one of --out or --brokers")

    events = iter_events(args.episodes, seed=args.seed, spread_seconds=args.spread_hours * 3600)
    n = 0
    if args.brokers:
        from kafka import KafkaProducer
        producer = KafkaProducer(bootstrap_servers=args.brokers,
                                  value_serializer=lambda v: json.dumps(v).encode())
        for e in events:
            producer.send(args.topic, e)
            n += 1
        producer.flush()
        producer.close()
    else:
        out = sys.stdout if args.out == "-" else open(args.out, "w", encoding="utf-8")
        try:
            for e in events:
                out.write(json.dumps(e) + "\n")
                n += 1
        finally:
            if out is not sys.stdout:
                out.close()
    print(f"generated {n} events ({args.episodes} episodes)", file=sys.stderr)


if __name__ == "__main__":
    main()
