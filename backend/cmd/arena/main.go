// Command arena is the headless evaluation server (contracts §4).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"zhuch/pkg/bots"
	"zhuch/pkg/brain"
	"zhuch/pkg/engine"
)

var mlpSizes = []int{82, 64, 64, 5} // contracts §3

type TankSpec struct {
	Name       string `json:"name"`
	WeightsB64 string `json:"weights_b64"`
	Scripted   bool   `json:"scripted"`
	Spawn      *struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"spawn"`
}

type EvalRequest struct {
	Seed            int64           `json:"seed"`
	MaxTicks        int             `json:"max_ticks"`
	ConfigOverrides json.RawMessage `json:"config_overrides"`
	Tanks           []TankSpec      `json:"tanks"`
}

type TankResult struct {
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	Kills     int     `json:"kills"`
	DeathTick int     `json:"death_tick"`
	Alive     bool    `json:"alive"`
}

type EvalResponse struct {
	Ticks int          `json:"ticks"`
	Tanks []TankResult `json:"tanks"`
}

// runEpisode plays one full-speed headless episode. tanks[0] is the
// candidate: the episode ends when it dies or at max_ticks.
func runEpisode(req EvalRequest) (*EvalResponse, error) {
	if len(req.Tanks) == 0 {
		return nil, errors.New("tanks required")
	}
	if req.MaxTicks <= 0 {
		return nil, errors.New("max_ticks must be > 0")
	}
	cfg := engine.DefaultConfig()
	if len(req.ConfigOverrides) > 0 {
		if err := json.Unmarshal(req.ConfigOverrides, &cfg); err != nil {
			return nil, fmt.Errorf("config_overrides: %w", err)
		}
	}
	g := engine.NewGameSeeded(cfg, req.Seed)

	n := len(req.Tanks)
	tanks := make([]*engine.Tank, n)
	agents := make([]*bots.Bot, n)
	for i, spec := range req.Tanks {
		var pos engine.Vector2
		if spec.Spawn != nil {
			pos = engine.Vector2{X: spec.Spawn.X, Y: spec.Spawn.Y}
		} else {
			// Seeded-random spawn with 100-unit wall margin (deterministic per seed).
			pos = engine.Vector2{
				X: 100 + g.Rng.Float64()*(cfg.MapWidth-200),
				Y: 100 + g.Rng.Float64()*(cfg.MapHeight-200),
			}
		}
		t := engine.NewTank(spec.Name, pos, &g.Config)
		g.SpawnTank(t)
		tanks[i] = t

		var pol bots.Policy
		if spec.Scripted {
			pol = &bots.Scripted{}
		} else {
			w, err := brain.DecodeWeightsB64(spec.WeightsB64)
			if err != nil {
				return nil, fmt.Errorf("tank %d (%s): %w", i, spec.Name, err)
			}
			m, err := brain.FromFlat(mlpSizes, w)
			if err != nil {
				return nil, fmt.Errorf("tank %d (%s): %w", i, spec.Name, err)
			}
			pol = bots.MLPPolicy{M: m}
		}
		agents[i] = &bots.Bot{Tank: t, Brain: pol, DecideEvery: 4, Offset: i % 4}
	}

	deathTick := make([]int, n)
	for i := range deathTick {
		deathTick[i] = -1
	}
	// No ticker, no sleep: bots run between ticks on this goroutine;
	// Game.Tick takes g.mu internally, which is fine.
	for tick := 0; tick < req.MaxTicks; tick++ {
		for _, a := range agents {
			a.Step(g, tick)
		}
		g.Tick()
		for i, t := range tanks {
			if deathTick[i] == -1 && !t.IsAlive() {
				deathTick[i] = g.CurrentTick
			}
		}
		if deathTick[0] != -1 {
			break // candidate died
		}
	}

	resp := &EvalResponse{Ticks: g.CurrentTick, Tanks: make([]TankResult, n)}
	for i, t := range tanks {
		resp.Tanks[i] = TankResult{
			Name:      req.Tanks[i].Name,
			Score:     t.Score,
			Kills:     t.Kills,
			DeathTick: deathTick[i],
			Alive:     deathTick[i] == -1,
		}
	}
	episodeFinished(resp)
	return resp, nil
}

// episodeFinished is the single episode-completion hook. The telemetry
// producer (WS-G, contracts §6) attaches here later.
var (
	epMu          sync.Mutex
	epCount       int
	epWindowStart = time.Now()
)

func episodeFinished(*EvalResponse) {
	epMu.Lock()
	epCount++
	if epCount%100 == 0 {
		now := time.Now()
		eps := 100 / now.Sub(epWindowStart).Seconds()
		epWindowStart = now
		slog.Info("arena throughput", "episodes_total", epCount, "episodes_per_sec", eps)
	}
	epMu.Unlock()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func handleEval(w http.ResponseWriter, r *http.Request) {
	var req EvalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := runEpisode(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, resp)
}

func handleEvalBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Jobs []EvalRequest `json:"jobs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	results := make([]*EvalResponse, len(req.Jobs))
	errs := make([]error, len(req.Jobs))
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := range req.Jobs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = runEpisode(req.Jobs[i])
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			http.Error(w, fmt.Sprintf("job %d: %v", i, err), http.StatusBadRequest)
			return
		}
	}
	writeJSON(w, map[string]any{"results": results})
}

func handleForward(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sizes      []int     `json:"sizes"`
		WeightsB64 string    `json:"weights_b64"`
		Input      []float64 `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	weights, err := brain.DecodeWeightsB64(req.WeightsB64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m, err := brain.FromFlat(req.Sizes, weights)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Input) != req.Sizes[0] {
		http.Error(w, fmt.Sprintf("input len %d, want %d", len(req.Input), req.Sizes[0]), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"output": m.Forward(req.Input)})
}

func main() {
	port := os.Getenv("ARENA_PORT")
	if port == "" {
		port = "8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /eval", handleEval)
	mux.HandleFunc("POST /eval_batch", handleEvalBatch)
	mux.HandleFunc("POST /forward", handleForward)

	slog.Info("arena listening", "port", port, "workers", runtime.GOMAXPROCS(0))
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("arena server exited", "err", err)
		os.Exit(1)
	}
}
