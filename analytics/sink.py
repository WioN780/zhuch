"""Kafka consumer -> hourly-partitioned Parquet sink for zhuch.events.v1.

out/dt=YYYY-MM-DD/hour=HH/part-<uuid>.parquet (partition derived from each event's
own "ts", i.e. event time, not wall-clock arrival time). Batched flush: every
--batch-size events or --flush-interval seconds, whichever comes first. Graceful
shutdown on SIGINT/SIGTERM flushes whatever is buffered before exiting.

--from-ndjson reads a local NDJSON file instead of Kafka, for offline dev/tests.

CLI:
    python -m analytics.sink --from-ndjson events.ndjson --out out
    python -m analytics.sink --brokers localhost:9092 --out out
"""
from __future__ import annotations

import argparse
import json
import signal
import sys
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable, Iterator, Optional

import pyarrow as pa
import pyarrow.parquet as pq

from analytics.synthetic import TOPIC

SCHEMA = pa.schema([
    pa.field("ts", pa.int64()),
    pa.field("source", pa.string()),
    pa.field("room", pa.string()),
    pa.field("episode", pa.string()),
    pa.field("type", pa.string()),
    pa.field("actor", pa.string()),
    pa.field("target", pa.string()),
    pa.field("pos", pa.struct([("x", pa.float64()), ("y", pa.float64())])),
    pa.field("data", pa.string()),  # json-encoded; shape varies by event type
])


def _partition(ts_ms: int) -> tuple[str, str]:
    dt = datetime.fromtimestamp(ts_ms / 1000, tz=timezone.utc)
    return dt.strftime("%Y-%m-%d"), dt.strftime("%H")


def _row(event: dict) -> dict:
    pos = event.get("pos")
    return {
        "ts": int(event["ts"]),
        "source": event.get("source"),
        "room": event.get("room"),
        "episode": event.get("episode"),
        "type": event["type"],
        "actor": event.get("actor"),
        "target": event.get("target"),
        "pos": {"x": float(pos["x"]), "y": float(pos["y"])} if pos else None,
        "data": json.dumps(event.get("data") or {}),
    }


class ParquetSink:
    """Buffers events by hourly partition, flushes on batch size or time elapsed."""

    def __init__(self, out_dir: Path, batch_size: int = 5000, flush_interval: float = 30.0):
        self.out_dir = Path(out_dir)
        self.batch_size = batch_size
        self.flush_interval = flush_interval
        self._buf: dict[tuple[str, str], list[dict]] = {}
        self._n_buffered = 0
        self._last_flush = time.monotonic()
        self.total_written = 0

    def add(self, event: dict) -> None:
        key = _partition(int(event["ts"]))
        self._buf.setdefault(key, []).append(_row(event))
        self._n_buffered += 1
        if self._n_buffered >= self.batch_size or self._due():
            self.flush()

    def _due(self) -> bool:
        return time.monotonic() - self._last_flush >= self.flush_interval

    def flush(self) -> None:
        self._last_flush = time.monotonic()
        if not self._buf:
            return
        for (dt, hour), rows in self._buf.items():
            part_dir = self.out_dir / f"dt={dt}" / f"hour={hour}"
            part_dir.mkdir(parents=True, exist_ok=True)
            table = pa.Table.from_pylist(rows, schema=SCHEMA)
            pq.write_table(table, part_dir / f"part-{uuid.uuid4().hex}.parquet")
            self.total_written += len(rows)
        self._buf.clear()
        self._n_buffered = 0


def _from_ndjson(path: str) -> Iterator[dict]:
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                yield json.loads(line)


def _from_kafka(brokers: list[str], topic: str, group: str) -> Iterator[dict]:
    from kafka import KafkaConsumer
    consumer = KafkaConsumer(
        topic, bootstrap_servers=brokers, group_id=group,
        value_deserializer=lambda v: json.loads(v.decode()),
        auto_offset_reset="earliest",
    )
    for msg in consumer:
        yield msg.value


def run(events: Iterable[dict], sink: ParquetSink, stop_after: Optional[int] = None) -> int:
    """Consume events into sink until exhausted, stop_after reached, or SIGINT/SIGTERM."""
    n = 0
    shutdown = {"flag": False}

    def _handle(signum, frame):
        shutdown["flag"] = True

    prev_int = signal.signal(signal.SIGINT, _handle)
    has_term = hasattr(signal, "SIGTERM")
    prev_term = signal.signal(signal.SIGTERM, _handle) if has_term else None
    try:
        for event in events:
            sink.add(event)
            n += 1
            if shutdown["flag"] or (stop_after and n >= stop_after):
                break
    finally:
        sink.flush()
        signal.signal(signal.SIGINT, prev_int)
        if has_term:
            signal.signal(signal.SIGTERM, prev_term)
    return n


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                  formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", default="out", help="output Parquet tree root")
    ap.add_argument("--brokers", nargs="+", help="Kafka bootstrap servers")
    ap.add_argument("--topic", default=TOPIC)
    ap.add_argument("--group", default="zhuch-sink")
    ap.add_argument("--from-ndjson", help="read events from a local NDJSON file instead of Kafka")
    ap.add_argument("--batch-size", type=int, default=5000)
    ap.add_argument("--flush-interval", type=float, default=30.0)
    ap.add_argument("--max-events", type=int, default=None, help="stop after N events (mainly for tests)")
    args = ap.parse_args(argv)
    if not args.from_ndjson and not args.brokers:
        ap.error("specify --from-ndjson or --brokers")

    events = (_from_ndjson(args.from_ndjson) if args.from_ndjson
              else _from_kafka(args.brokers, args.topic, args.group))
    sink = ParquetSink(Path(args.out), batch_size=args.batch_size, flush_interval=args.flush_interval)
    n = run(events, sink, stop_after=args.max_events)
    print(f"sank {n} events -> {sink.total_written} rows written under {args.out}", file=sys.stderr)


if __name__ == "__main__":
    main()
