from analytics.spark_jobs.difficulty import MULTIPLIER_BOUNDS, suggest


def test_suggest_neutral_at_target():
    assert suggest(0.5) == 1.0


def test_suggest_stays_within_bounds():
    lo, hi = MULTIPLIER_BOUNDS
    assert lo <= suggest(1.0) <= hi
    assert lo <= suggest(0.0) <= hi


def test_suggest_direction():
    # bots winning too much -> weaken (multiplier < 1); too little -> strengthen (> 1)
    assert suggest(0.9) < 1.0
    assert suggest(0.1) > 1.0
