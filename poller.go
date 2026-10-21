package main

import (
	"context"
	"sync"
	"time"
)

// Source is a backend that can return the current set of channel values. It is
// implemented by both the HTTP/JSON and the Modbus/TCP backends.
type Source interface {
	Poll(ctx context.Context) ([]Value, error)
}

// Value is one merged DAQ channel: descriptor metadata plus the current value.
type Value struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Unit  string `json:"unit"`
	Value any    `json:"value"`
}

// Snapshot is the latest poll result served to clients.
type Snapshot struct {
	Updated time.Time `json:"updated"`
	Err     string    `json:"error,omitempty"`
	Values  []Value   `json:"values"`
}

// sampleSink receives each successful poll for recording (the history store).
type sampleSink interface {
	Add(ts time.Time, vals []Value)
}

// Poller periodically fetches channel data from a Source and keeps the latest
// Snapshot in memory for serving.
type Poller struct {
	src      Source
	interval time.Duration
	sinks    []sampleSink // optional; each fed on a successful poll (history, UDP, …)

	mu   sync.RWMutex
	snap Snapshot
}

func NewPoller(src Source, interval time.Duration) *Poller {
	return &Poller{src: src, interval: interval}
}

// AddSink registers a sink to receive every successful poll.
func (p *Poller) AddSink(s sampleSink) {
	if s != nil {
		p.sinks = append(p.sinks, s)
	}
}

// Snapshot returns a copy of the latest snapshot.
func (p *Poller) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap
}

// Run polls until ctx is cancelled. It does one poll immediately, then on the ticker.
func (p *Poller) Run(ctx context.Context) {
	p.pollOnce(ctx)
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.pollOnce(ctx)
		}
	}
}

func (p *Poller) pollOnce(ctx context.Context) {
	vals, err := p.src.Poll(ctx)
	if err != nil {
		p.store(Snapshot{Updated: time.Now(), Err: err.Error()})
		return
	}
	now := time.Now()
	p.store(Snapshot{Updated: now, Values: vals})
	for _, sink := range p.sinks {
		sink.Add(now, vals)
	}
}

func (p *Poller) store(s Snapshot) {
	p.mu.Lock()
	p.snap = s
	p.mu.Unlock()
}
