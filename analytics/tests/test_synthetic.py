from analytics.synthetic import MODES, iter_events


def test_iter_events_shape():
    events = list(iter_events(n_episodes=5, seed=7))
    assert events

    types = {e["type"] for e in events}
    assert {"spawn", "pos_sample", "episode_end"} <= types
    assert types <= {"spawn", "kill", "death", "damage", "pos_sample", "episode_end"}

    for e in events:
        assert isinstance(e["ts"], int)
        assert e["source"] and e["room"] and e["episode"]
        assert "mode" in e["data"] and e["data"]["mode"] in MODES

    ends = [e for e in events if e["type"] == "episode_end"]
    assert len(ends) == 5
    for e in ends:
        assert "ticks" in e["data"] and "tanks" in e["data"]
        for t in e["data"]["tanks"]:
            assert t["name"].startswith(("bot-", "human-"))


def test_iter_events_deterministic_with_seed():
    a = list(iter_events(n_episodes=3, seed=42))
    b = list(iter_events(n_episodes=3, seed=42))
    assert [e["episode"] for e in a] == [e["episode"] for e in b]
