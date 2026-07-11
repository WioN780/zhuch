# Bot models

Weight files for `backend/internal/bots` (loaded via `backend/pkg/brain.LoadModel`).
A room's `bot_model` field (default `"champion"`) names a file here:
`backend/models/<bot_model>.json`.

## Format (contracts §3)

Single JSON document:

```json
{"format": "zhuch-mlp", "version": 1, "sizes": [82,64,64,5],
 "sha256": "<hex sha256 of the raw little-endian float32 weight bytes>",
 "weights_b64": "<base64 of those bytes>"}
```

- Architecture: sizes `[82, 64, 64, 5]`, tanh on hidden layers, identity output.
- Weights: flat, layer-major (`W` row-major shape (out,in), then bias `b`, per
  layer), stored as float32 little-endian, base64-encoded in `weights_b64`.
- `sha256` is checked against the decoded weight bytes on load; a mismatch is
  a load error, not a silent fallback.

## `champion.json`

Not checked in here. It's produced by the training pipeline
(`training/scripts/`) via an MLflow registry export of the current best
genome, converted to this JSON format and dropped into this directory.

If `champion.json` (or whatever `bot_model` names) is missing or fails to
load, `internal/bots.NewBotSet` logs a warning and falls back to the frozen
`ScriptedPolicy` (`backend/pkg/bots/scripted.go`, contracts §9), so rooms
still get playable bots without a trained model present.
