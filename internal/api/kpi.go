package api

// Auswertung key figures as flat values for the plain-text surfaces (Loxone, metrics).
// The full payload stays available as JSON at /api/insights; this exposes the numbers
// a Miniserver or Prometheus typically wants, with stable keys (0 when no data yet).

type kpi struct {
	Key    string // Loxone key (letters/digits only)
	Metric string // Prometheus suffix: guntamatic_insight_<Metric>
	Title  string // German label for templates/help
	Unit   string
	Value  float64
}

// kpiDays is the window of the "7 Tage" aggregates.
const kpiDays = 7

// ratingCode maps the cycling grade to a number: 0 = no runs, 1 good, 2 fair, 3 poor.
var ratingCode = map[string]float64{"good": 1, "fair": 2, "poor": 3}

// insightKPIs returns today's, yesterday's and the 7-day key figures, or nil when the
// history is disabled.
func (s *Server) insightKPIs() []kpi {
	if s.history == nil {
		return nil
	}
	in := s.history.Insights(kpiDays)
	n := len(in.Days)
	if n < 2 {
		return nil
	}
	today, yday := in.Days[n-1], in.Days[n-2]
	f := func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	}
	i := func(p *int) float64 {
		if p == nil {
			return 0
		}
		return float64(*p)
	}
	sum := in.Summary
	return []kpi{
		{"PelletsHeute", "pellets_today_kg", "Pellets heute", "kg", f(today.PelletsKg)},
		{"PelletsGestern", "pellets_yesterday_kg", "Pellets gestern", "kg", f(yday.PelletsKg)},
		{"WaermeHeute", "heat_today_kwh", "Wärme heute", "kWh", f(today.HeatKWh)},
		{"WaermeGestern", "heat_yesterday_kwh", "Wärme gestern", "kWh", f(yday.HeatKWh)},
		{"StartsHeute", "starts_today", "Brennerstarts heute", "", i(today.Starts)},
		{"StartsGestern", "starts_yesterday", "Brennerstarts gestern", "", i(yday.Starts)},
		{"LaufzeitHeute", "runtime_today_hours", "Laufzeit heute", "h", f(today.RuntimeHours)},
		{"LaufzeitGestern", "runtime_yesterday_hours", "Laufzeit gestern", "h", f(yday.RuntimeHours)},
		{"TaktGestern", "cycle_yesterday_minutes", "Ø Laufzeit pro Start gestern", "min", f(yday.AvgCycleMin)},
		{"TaktBewertungGestern", "cycle_rating_yesterday", "Takt-Bewertung gestern (0 –, 1 gut, 2 mittel, 3 schlecht)", "", ratingCode[yday.Rating]},
		{"HGTGestern", "degree_days_yesterday", "Heizgradtage gestern", "Kd", f(yday.DegreeDays)},
		{"Pellets7Tage", "pellets_7d_kg", "Pellets 7 Tage (vollständige Tage)", "kg", sum.PelletsKg},
		{"Pellets7TageProTag", "pellets_7d_kg_per_day", "Pellets Ø pro Tag (7 Tage)", "kg", f(sum.KgPerDay)},
		{"Pellets7TageProHGT", "pellets_7d_kg_per_degree_day", "Pellets pro Heizgradtag (7 Tage)", "kg/Kd", f(sum.KgPerDegree)},
		{"Starts7TageProTag", "starts_7d_per_day", "Brennerstarts Ø pro Tag (7 Tage)", "", f(sum.StartsPerDay)},
	}
}
