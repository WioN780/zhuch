// Package telemetry publishes game events to Kafka (topic zhuch.events.v1,
// schema docs/contracts.md §6). Emission is gated on the KAFKA_BROKERS env
// var: unset means every call is a no-op, so the game and arena run fine
// without any broker.
package telemetry

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const Topic = "zhuch.events.v1"

type Pos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Event is the wire format of schema v1.
type Event struct {
	Ts      int64          `json:"ts"`
	Source  string         `json:"source"`
	Room    string         `json:"room"`
	Episode string         `json:"episode,omitempty"`
	Type    string         `json:"type"`
	Actor   string         `json:"actor"`
	Target  string         `json:"target,omitempty"`
	Pos     *Pos           `json:"pos,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type Producer struct {
	client *kgo.Client
	source string
}

// New returns a producer for the given source ("live" or "arena"). When
// KAFKA_BROKERS is unset the producer is inert (Emit is a no-op).
func New(source string) *Producer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		return &Producer{source: source}
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(brokers, ",")...),
		kgo.DefaultProduceTopic(Topic),
		kgo.AllowAutoTopicCreation(),
		// Tiny JSON records on a local broker: compression buys nothing and
		// the default snappy codec breaks pure-python consumers.
		kgo.ProducerBatchCompression(kgo.NoCompression()),
	)
	if err != nil {
		slog.Error("telemetry disabled: kafka client failed", "error", err)
		return &Producer{source: source}
	}
	slog.Info("telemetry enabled", "brokers", brokers, "topic", Topic)
	return &Producer{client: client, source: source}
}

// Emit publishes one event asynchronously. Safe to call from the game tick:
// TryProduce never blocks — when the buffer is full (broker slow or
// unreachable) the event is dropped. Telemetry must never stall the sim;
// a blocking Produce here collapsed arena throughput ~150x.
func (p *Producer) Emit(ev Event) {
	if p == nil || p.client == nil {
		return
	}
	ev.Ts = time.Now().UnixMilli()
	ev.Source = p.source
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	p.client.TryProduce(context.Background(), &kgo.Record{Value: payload}, nil)
}

// Close flushes pending records.
func (p *Producer) Close() {
	if p == nil || p.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.client.Flush(ctx)
	p.client.Close()
}
