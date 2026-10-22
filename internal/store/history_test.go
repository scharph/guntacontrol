package store

import (
	"math"
	"testing"
	"time"
)

func TestRateCycling(t *testing.T) {
	cases := []struct {
		starts int
		cycle  float64
		want   string
	}{
		{0, 0, ""},
		{3, 180, "good"},
		{5, 120, "good"},
		{8, 150, "fair"},  // starts in the middle band
		{4, 90, "fair"},   // cycle in the middle band
		{12, 150, "poor"}, // too many starts
		{2, 45, "poor"},   // runs too short
	}
	for _, c := range cases {
		if got := rateCycling(c.starts, c.cycle); got != c.want {
			t.Errorf("rateCycling(%d, %v) = %q, want %q", c.starts, c.cycle, got, c.want)
		}
	}
}

// testHistory builds a History over one synthetic day without a database: the burner
// runs at runPct for the first runFor of every period, outdoor is constant.
func testHistory(day time.Time, period, runFor time.Duration, runPct, outdoor float64) *History {
	idx := make(map[string]int, len(HistoryChannels))
	for i, c := range HistoryChannels {
		idx[c] = i
	}
	h := &History{
		interval:  time.Minute,
		retention: 48 * time.Hour,
		idx:       idx,
		pellets:   PelletModel{NominalKW: 20, Efficiency: 0.8, KWhPerKg: 5},
	}
	for ts := day; ts.Before(day.Add(24 * time.Hour)); ts = ts.Add(time.Minute) {
		row := make([]float64, len(HistoryChannels))
		for i := range row {
			row[i] = math.NaN()
		}
		row[idx["Aussentemperatur"]] = outdoor
		row[idx[runningChannel]] = 0
		if ts.Sub(day)%period < runFor {
			row[idx[runningChannel]] = runPct
		}
		h.samples = append(h.samples, sample{T: ts, V: row})
	}
	return h
}

func TestDayInsightPellets(t *testing.T) {
	day := time.Date(2026, 1, 10, 0, 0, 0, 0, time.Local)
	// 4 runs of 3 h at 50 % → 12 h × 10 kW = 120 kWh heat → 150 kWh fuel → 30 kg.
	h := testHistory(day, 6*time.Hour, 3*time.Hour, 50, 2)
	d := h.dayInsight(day, day.Add(24*time.Hour))

	if d.Starts == nil || *d.Starts != 3 { // the first run starts with the day, not counted
		t.Fatalf("starts = %v, want 3", d.Starts)
	}
	if d.HeatKWh == nil || math.Abs(*d.HeatKWh-120) > 0.5 {
		t.Errorf("heat = %v, want ≈120 kWh", d.HeatKWh)
	}
	if d.PelletsKg == nil || math.Abs(*d.PelletsKg-30) > 0.2 {
		t.Errorf("pellets = %v, want ≈30 kg", d.PelletsKg)
	}
	if d.DegreeDays == nil || *d.DegreeDays != 18 {
		t.Errorf("degree days = %v, want 18", d.DegreeDays)
	}
	if d.Rating != "good" {
		t.Errorf("rating = %q, want good", d.Rating)
	}
}

func TestDegreeDaysAboveHeatingLimit(t *testing.T) {
	day := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	h := testHistory(day, 24*time.Hour, 30*time.Minute, 60, 18)
	d := h.dayInsight(day, day.Add(24*time.Hour))
	if d.DegreeDays == nil || *d.DegreeDays != 0 {
		t.Errorf("degree days = %v, want 0 above the heating limit", d.DegreeDays)
	}
}

func TestSummarize(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	i := func(v int) *int { return &v }
	days := []DayInsight{
		{Coverage: 1, PelletsKg: f(20), DegreeDays: f(15), Starts: i(4), RuntimeHours: f(10)},
		{Coverage: 1, PelletsKg: f(30), DegreeDays: f(25), Starts: i(6), RuntimeHours: f(14)},
		{Coverage: 0.4, PelletsKg: f(99), DegreeDays: f(99), Starts: i(99)}, // incomplete: ignored
	}
	s := summarize(days)
	if s.Days != 2 || s.PelletsKg != 50 || s.DegreeDays != 40 {
		t.Fatalf("summary = %+v", s)
	}
	if *s.KgPerDay != 25 || *s.KgPerDegree != 1.25 || *s.StartsPerDay != 5 || *s.AvgCycleMin != 144 {
		t.Errorf("kg/day=%v kg/HGT=%v starts/day=%v cycle=%v", *s.KgPerDay, *s.KgPerDegree, *s.StartsPerDay, *s.AvgCycleMin)
	}
}
