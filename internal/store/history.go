package store

import (
	"database/sql"
	"log"
	"math"
	"strconv"
	"sync"
	"time"

	"guntacontrol/internal/boiler"
)

// HistoryChannels are the numeric daqdata channels recorded for the Auswertung.
// Order is fixed: each in-memory sample stores values in this order.
var HistoryChannels = []string{
	"Aussentemperatur",
	"Kesseltemperatur",
	"Puffer oben",
	"Puffer unten",
	"Warmwasser 0",
	"Leistung",
	"Pufferladung",
	// Flow temperatures per heating circuit — recorded for the heating-curve view
	// (outdoor temperature vs. actual flow).
	"Vorlauf Ist 0",
	"Vorlauf Ist 1",
	"Vorlauf Ist 2",
	"Vorlauf Ist 3",
	"Vorlauf Ist 4",
}

// runningChannel drives the burner-running gate (starts, runtime, cycle length):
// the boiler is considered to be firing while its modulation is above zero.
const runningChannel = "Leistung"

// sample is one recorded poll in HistoryChannels order (NaN marks absent values).
type sample struct {
	T time.Time
	V []float64
}

// History is a continuous time series backed by SQLite (table "samples"). For fast
// chart/insight computation it also keeps the retention window in an in-memory ring,
// loaded from the database on start and kept in sync on every Add.
type History struct {
	mu        sync.RWMutex
	interval  time.Duration
	retention time.Duration
	db        *sql.DB
	idx       map[string]int // channel -> position in sample.V
	samples   []sample
}

func NewHistory(db *sql.DB, interval, retention time.Duration) *History {
	idx := make(map[string]int, len(HistoryChannels))
	for i, c := range HistoryChannels {
		idx[c] = i
	}
	h := &History{
		interval:  interval,
		retention: retention,
		db:        db,
		idx:       idx,
	}
	h.load()
	return h
}

// Add records one poll's values. Only HistoryChannels are kept, as float64 (bool
// EIN/AUS becomes 1/0). Rows go to SQLite and the in-memory ring.
func (h *History) Add(ts time.Time, vals []boiler.Value) {
	row := make([]float64, len(HistoryChannels))
	for i := range row {
		row[i] = math.NaN()
	}
	for _, v := range vals {
		pos, ok := h.idx[v.Name]
		if !ok {
			continue
		}
		switch x := v.Value.(type) {
		case float64:
			row[pos] = x
		case bool:
			if x {
				row[pos] = 1
			} else {
				row[pos] = 0
			}
		}
	}
	h.persist(ts, row)

	h.mu.Lock()
	h.samples = append(h.samples, sample{T: ts, V: row})
	cutoff := ts.Add(-h.retention)
	if len(h.samples) > 0 && h.samples[0].T.Before(cutoff) {
		keep := 0
		for keep < len(h.samples) && h.samples[keep].T.Before(cutoff) {
			keep++
		}
		h.samples = append(h.samples[:0], h.samples[keep:]...)
	}
	h.mu.Unlock()
}

// persist writes the non-NaN channels of one sample to SQLite in a single transaction.
func (h *History) persist(ts time.Time, row []float64) {
	if h.db == nil {
		return
	}
	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("history: begin failed: %v", err)
		return
	}
	stmt, err := tx.Prepare(`INSERT INTO samples(ts, channel, value) VALUES(?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		log.Printf("history: prepare failed: %v", err)
		return
	}
	ms := ts.UnixMilli()
	for i, v := range row {
		if math.IsNaN(v) {
			continue
		}
		if _, err := stmt.Exec(ms, HistoryChannels[i], v); err != nil {
			stmt.Close()
			tx.Rollback()
			log.Printf("history: insert failed: %v", err)
			return
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		log.Printf("history: commit failed: %v", err)
	}
}

// ---- chart series ----

// Point is one charted sample (x = unix millis, y = value).
type Point struct {
	X int64   `json:"x"`
	Y float64 `json:"y"`
}

// Series is one channel's downsampled points over a time range.
type Series struct {
	Channel string  `json:"channel"`
	Unit    string  `json:"unit"`
	Points  []Point `json:"points"`
}

// Range returns up to maxPoints per channel over [from, to], bucket-averaged.
func (h *History) Range(from, to time.Time, channels []string, maxPoints int, units map[string]string) []Series {
	if maxPoints <= 0 {
		maxPoints = 500
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	span := to.Sub(from)
	if span <= 0 {
		return nil
	}
	bucket := span / time.Duration(maxPoints)
	if bucket <= 0 {
		bucket = time.Millisecond
	}

	type acc struct {
		sum float64
		n   int
	}
	out := make([]Series, 0, len(channels))
	for _, ch := range channels {
		pos, ok := h.idx[ch]
		if !ok {
			continue
		}
		buckets := make(map[int64]*acc)
		for _, s := range h.samples {
			if s.T.Before(from) || s.T.After(to) {
				continue
			}
			y := s.V[pos]
			if math.IsNaN(y) {
				continue
			}
			key := int64(s.T.Sub(from) / bucket)
			a := buckets[key]
			if a == nil {
				a = &acc{}
				buckets[key] = a
			}
			a.sum += y
			a.n++
		}
		pts := make([]Point, 0, len(buckets))
		for key, a := range buckets {
			t := from.Add(time.Duration(key)*bucket + bucket/2)
			pts = append(pts, Point{X: t.UnixMilli(), Y: round1(a.sum / float64(a.n))})
		}
		sortPoints(pts)
		out = append(out, Series{Channel: ch, Unit: units[ch], Points: pts})
	}
	return out
}

func sortPoints(p []Point) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && p[j-1].X > p[j].X; j-- {
			p[j-1], p[j] = p[j], p[j-1]
		}
	}
}

// ---- daily insights ----

// DayInsight summarizes one calendar day; nil fields had no data.
type DayInsight struct {
	Date         string   `json:"date"`
	Coverage     float64  `json:"coverage"`
	Starts       *int     `json:"starts,omitempty"`
	RuntimeHours *float64 `json:"runtimeHours,omitempty"`
	AvgCycleMin  *float64 `json:"avgCycleMinutes,omitempty"`
	Outdoor      *float64 `json:"outdoor,omitempty"`
	KesselRun    *float64 `json:"kesselRunning,omitempty"`
	PowerRun     *float64 `json:"powerRunning,omitempty"`
	BufferTop    *float64 `json:"bufferTop,omitempty"`
}

// Hint is a rule-based observation about recent operation.
type Hint struct {
	Level  string `json:"level"` // info | warn
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// Insights is the Auswertung payload: per-day rows plus hints.
type Insights struct {
	Days  []DayInsight `json:"days"`
	Hints []Hint       `json:"hints"`
}

// Insights computes the last `days` calendar days (today included) and a few hints.
func (h *History) Insights(days int) Insights {
	if days < 1 {
		days = 1
	}
	if days > 90 {
		days = 90
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	out := Insights{Days: []DayInsight{}, Hints: []Hint{}}
	for i := days - 1; i >= 0; i-- {
		from := today.AddDate(0, 0, -i)
		to := from.AddDate(0, 0, 1)
		if to.After(now) {
			to = now
		}
		out.Days = append(out.Days, h.dayInsight(from, to))
	}
	out.Hints = h.hints(out.Days)
	return out
}

// stale caps how long one sample's value is trusted when the next is missing.
func (h *History) stale() time.Duration {
	s := 3*h.interval + time.Minute
	if s < 2*time.Minute {
		s = 2 * time.Minute
	}
	return s
}

func (h *History) dayInsight(from, to time.Time) DayInsight {
	d := DayInsight{Date: from.Format("2006-01-02")}
	h.mu.RLock()
	defer h.mu.RUnlock()

	posRun := h.idx[runningChannel]
	posOut := h.idx["Aussentemperatur"]
	posKes := h.idx["Kesseltemperatur"]
	posBuf := h.idx["Puffer oben"]
	stale := h.stale()

	var covered, runDur time.Duration
	var starts int
	var outSum, outDur float64
	var kesSum, powSum, powDur float64
	var bufSum, bufDur float64
	prevRunning := false
	havePrev := false

	within := make([]sample, 0, 512)
	for _, s := range h.samples {
		if s.T.Before(from) || !s.T.Before(to) {
			continue
		}
		within = append(within, s)
	}
	for i, s := range within {
		segEnd := to
		if i+1 < len(within) {
			if within[i+1].T.Before(segEnd) {
				segEnd = within[i+1].T
			}
		}
		if lim := s.T.Add(stale); segEnd.After(lim) {
			segEnd = lim
		}
		dur := segEnd.Sub(s.T)
		if dur < 0 {
			dur = 0
		}
		covered += dur

		running := !math.IsNaN(s.V[posRun]) && s.V[posRun] > 0
		if havePrev && running && !prevRunning {
			starts++
		}
		if running {
			runDur += dur
			if !math.IsNaN(s.V[posKes]) {
				kesSum += s.V[posKes] * dur.Seconds()
			}
			if !math.IsNaN(s.V[posRun]) {
				powSum += s.V[posRun] * dur.Seconds()
			}
			powDur += dur.Seconds()
		}
		if !math.IsNaN(s.V[posOut]) {
			outSum += s.V[posOut] * dur.Seconds()
			outDur += dur.Seconds()
		}
		if !math.IsNaN(s.V[posBuf]) {
			bufSum += s.V[posBuf] * dur.Seconds()
			bufDur += dur.Seconds()
		}
		prevRunning, havePrev = running, true
	}

	dayLen := to.Sub(from)
	if dayLen > 0 {
		d.Coverage = clamp01(covered.Seconds() / dayLen.Seconds())
	}
	if len(within) == 0 {
		return d
	}
	st := starts
	d.Starts = &st
	rh := round1(runDur.Hours())
	d.RuntimeHours = &rh
	if starts > 0 {
		c := round1(runDur.Minutes() / float64(starts))
		d.AvgCycleMin = &c
	}
	if outDur > 0 {
		d.Outdoor = ptrf(round1(outSum / outDur))
	}
	if powDur > 0 {
		d.KesselRun = ptrf(round1(kesSum / powDur))
		d.PowerRun = ptrf(round1(powSum / powDur))
	}
	if bufDur > 0 {
		d.BufferTop = ptrf(round1(bufSum / bufDur))
	}
	return d
}

func (h *History) hints(days []DayInsight) []Hint {
	hints := []Hint{}
	var recent []DayInsight
	for i := len(days) - 1; i >= 0 && len(recent) < 7; i-- {
		if days[i].Coverage > 0 {
			recent = append(recent, days[i])
		}
	}
	if len(recent) == 0 {
		hints = append(hints, Hint{
			Level:  "info",
			Title:  "Datensammlung läuft",
			Detail: "Noch keine Verlaufsdaten – die Auswertung füllt sich, während der Server läuft.",
		})
		return hints
	}

	for _, d := range recent {
		if d.Coverage >= 0.5 && d.Starts != nil && *d.Starts >= 10 && d.AvgCycleMin != nil && *d.AvgCycleMin < 20 {
			hints = append(hints, Hint{
				Level:  "warn",
				Title:  "Häufiges Takten",
				Detail: "Am " + d.Date + ": " + itoa(*d.Starts) + " Starts bei Ø " + ftoa(*d.AvgCycleMin) + " min Laufzeit. Kurze Takte senken Wirkungsgrad und Lebensdauer.",
			})
			break
		}
	}

	if recent[0].Coverage < 0.5 {
		hints = append(hints, Hint{
			Level:  "info",
			Title:  "Lückenhafte Daten",
			Detail: "Für " + recent[0].Date + " liegen nur " + ftoa(recent[0].Coverage*100) + " % Abdeckung vor (Server war zeitweise aus).",
		})
	}

	if len(hints) == 0 {
		hints = append(hints, Hint{
			Level:  "info",
			Title:  "Unauffälliger Betrieb",
			Detail: "In den letzten Tagen keine Auffälligkeiten (Takten, Datenlücken) erkannt.",
		})
	}
	return hints
}

// ---- persistence (SQLite) ----

// load reads the retention window from SQLite into the in-memory ring.
func (h *History) load() {
	if h.db == nil {
		return
	}
	cutoff := time.Now().Add(-h.retention).UnixMilli()
	rows, err := h.db.Query(
		`SELECT ts, channel, value FROM samples WHERE ts >= ? ORDER BY ts`, cutoff)
	if err != nil {
		log.Printf("history: load query failed: %v", err)
		return
	}
	defer rows.Close()

	var samples []sample
	var cur *sample
	var curMs int64 = -1
	for rows.Next() {
		var ms int64
		var ch string
		var val float64
		if err := rows.Scan(&ms, &ch, &val); err != nil {
			log.Printf("history: scan failed: %v", err)
			return
		}
		if ms != curMs {
			row := make([]float64, len(HistoryChannels))
			for i := range row {
				row[i] = math.NaN()
			}
			samples = append(samples, sample{T: time.UnixMilli(ms), V: row})
			cur = &samples[len(samples)-1]
			curMs = ms
		}
		if pos, ok := h.idx[ch]; ok {
			cur.V[pos] = val
		}
	}
	h.samples = samples
	log.Printf("history: loaded %d samples from sqlite", len(h.samples))
}

// prune deletes samples older than the retention window from SQLite.
func (h *History) prune() {
	if h.db == nil {
		return
	}
	cutoff := time.Now().Add(-h.retention).UnixMilli()
	if _, err := h.db.Exec(`DELETE FROM samples WHERE ts < ?`, cutoff); err != nil {
		log.Printf("history: prune failed: %v", err)
	}
}

// RunMaintenance prunes the database periodically until stop is closed.
func (h *History) RunMaintenance(stop <-chan struct{}, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			h.prune()
		}
	}
}

// ---- small helpers ----

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func ptrf(v float64) *float64  { return &v }
func itoa(v int) string        { return strconv.Itoa(v) }
func ftoa(v float64) string    { return strconv.FormatFloat(v, 'f', 1, 64) }
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
