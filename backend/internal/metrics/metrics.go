// Package metrics holds the Prometheus metric names pinned by
// docs/contracts.md §8, shared by the game server (cmd/server) and the
// arena (cmd/arena). Names/labels here are load-bearing: existing Grafana
// dashboards assume them exactly.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Game server metrics (contracts §8).
	TickDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "zhuch_tick_duration_seconds",
		Help:    "Game tick processing duration.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
	}, []string{"room"})

	Entities = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zhuch_entities",
		Help: "Current entity count.",
	}, []string{"room"})

	Players = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zhuch_players",
		Help: "Current connected player count.",
	}, []string{"room"})

	Bots = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zhuch_bots",
		Help: "Current bot tank count.",
	}, []string{"room"})

	Rooms = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zhuch_rooms",
		Help: "Current active room count.",
	})

	Kills = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "zhuch_kills_total",
		Help: "Total tank kills.",
	}, []string{"room"})

	Deaths = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "zhuch_deaths_total",
		Help: "Total tank deaths.",
	}, []string{"room"})

	WSDisconnects = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "zhuch_ws_disconnects_total",
		Help: "Total client websocket disconnects.",
	}, []string{"room"})

	// Arena metrics (contracts §8), no labels.
	ArenaEpisodes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arena_episodes_total",
		Help: "Total evaluation episodes completed.",
	})

	ArenaEpisodeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "arena_episode_duration_seconds",
		Help: "Wall-clock duration of one evaluation episode.",
	})

	ArenaTicks = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arena_ticks_total",
		Help: "Total ticks simulated across all episodes.",
	})

	ArenaInflightEvals = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arena_inflight_evals",
		Help: "Evaluation episodes currently running.",
	})

	ArenaEvalErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arena_eval_errors_total",
		Help: "Total /eval and /eval_batch requests that returned an error.",
	})
)

func init() {
	prometheus.MustRegister(
		TickDuration, Entities, Players, Bots, Rooms, Kills, Deaths, WSDisconnects,
		ArenaEpisodes, ArenaEpisodeDuration, ArenaTicks, ArenaInflightEvals, ArenaEvalErrors,
	)
}

// Handler serves the Prometheus text exposition format for GET /metrics.
func Handler() http.Handler { return promhttp.Handler() }
