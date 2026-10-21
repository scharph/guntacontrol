// Package service polls the boiler and keeps the latest snapshot in memory, fanning
// each successful poll out to registered sinks (history, Loxone UDP, …).
package service

import (
	"context"
	"sync"
	"time"

	"guntacontrol/internal/boiler"
)

// Source is a backend that can return the current set of channel values.
type Source interface {
	Poll(ctx context.Context) ([]boiler.Value, error)
}

// SampleSink receives each successful poll for recording or forwarding.
type SampleSink interface {
	Add(ts time.Time, vals []boiler.Value)
}

// Poller periodically fetches channel data from a Source and keeps the latest snapshot.
type Poller struct {
	src      Source
	interval time.Duration
	sinks    []SampleSink

	mu   sync.RWMutex
	snap boiler.Snapshot
}

func NewPoller(src Source, interval time.Duration) *Poller {
	return &Poller{src: src, interval: interval}
}

// AddSink registers a sink to receive every successful poll.
func (p *Poller) AddSink(s SampleSink) {
	if s != nil {
		p.sinks = append(p.sinks, s)
	}
}

// Snapshot returns a copy of the latest snapshot.
func (p *Poller) Snapshot() boiler.Snapshot {
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
		p.store(boiler.Snapshot{Updated: time.Now(), Err: err.Error()})
		return
	}
	now := time.Now()
	p.store(boiler.Snapshot{Updated: now, Values: vals})
	for _, sink := range p.sinks {
		sink.Add(now, vals)
	}
}

func (p *Poller) store(s boiler.Snapshot) {
	p.mu.Lock()
	p.snap = s
	p.mu.Unlock()
}
