package device

import (
	"context"
	"strconv"
	"strings"

	"guntacontrol/internal/boiler"
)

// Time programs ("Uhrenprogramme") are exposed through par.cgi as type-9 parameters.
// Each encodes a full week as 7 days of 9 integers:
//
//	[start1, start2, start3, end1, end2, end3, 0, 0, 0]
//
// i.e. up to three heating/charging windows per day, as minutes since midnight
// (360 = 06:00, 1320 = 22:00). A window is active when end > start; zeros mean unused.
// Heating circuits carry three weather variants (base = Normal/cold, "…b" = mild,
// "…c" = warm).

// weekdays follows the controller's Monday-first order.
var weekdays = []string{"Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"}

type variantSpec struct{ id, label string }

type scheduleSpec struct {
	title    string
	kind     string
	variants []variantSpec
}

// scheduleSpecs lists, in display order, the time programs we surface.
var scheduleSpecs = []scheduleSpec{
	{"Heizkreis 0", "heating", []variantSpec{{"HK003", "Normal"}, {"HK003b", "mild"}, {"HK003c", "warm"}}},
	{"Heizkreis 1", "heating", []variantSpec{{"HK103", "Normal"}, {"HK103b", "mild"}, {"HK103c", "warm"}}},
	{"Heizkreis 2", "heating", []variantSpec{{"HK203", "Normal"}, {"HK203b", "mild"}, {"HK203c", "warm"}}},
	{"Warmwasser", "water", []variantSpec{{"BK002", "Normal"}, {"BK003", "Sommer"}}},
	{"WW-Zirkulation", "circulation", []variantSpec{{"ZK002", "Normal"}, {"ZK003", "Sommer"}}},
}

// parseParamInts returns the leading integer fields of a par.cgi line (after the id and
// type columns), stopping at the first non-integer token (the human-readable label).
func parseParamInts(line string) (id string, ok bool, vals []int) {
	f := strings.Split(line, ";")
	if len(f) < 3 {
		return "", false, nil
	}
	id = strings.TrimSpace(f[0])
	for _, tok := range f[2:] {
		n, err := strconv.Atoi(strings.TrimSpace(tok))
		if err != nil {
			break
		}
		vals = append(vals, n)
	}
	return id, len(vals) >= 63, vals
}

// decodeClockProgram turns the 7×9 integer block into per-day active windows.
func decodeClockProgram(vals []int) []boiler.DaySchedule {
	days := make([]boiler.DaySchedule, len(weekdays))
	for d := range weekdays {
		base := d * 9
		day := boiler.DaySchedule{Day: weekdays[d], Windows: []boiler.TimeWindow{}}
		for i := 0; i < 3; i++ {
			start, end := vals[base+i], vals[base+3+i]
			if end > start { // zeros / unset slots collapse away
				day.Windows = append(day.Windows, boiler.TimeWindow{Start: start, End: end})
			}
		}
		days[d] = day
	}
	return days
}

// buildSchedules decodes the configured schedules from a par.cgi parameter map.
func buildSchedules(params map[string][]int) []boiler.Schedule {
	var out []boiler.Schedule
	for _, spec := range scheduleSpecs {
		var variants []boiler.ScheduleVariant
		for _, v := range spec.variants {
			vals, ok := params[v.id]
			if !ok {
				continue
			}
			variants = append(variants, boiler.ScheduleVariant{
				ID:    v.id,
				Label: v.label,
				Days:  decodeClockProgram(vals),
			})
		}
		if len(variants) > 0 {
			out = append(out, boiler.Schedule{Title: spec.title, Kind: spec.kind, Variants: variants})
		}
	}
	return out
}

// Schedules fetches par.cgi and returns the decoded time programs. It is read-only.
func (b *FlexBackend) Schedules(ctx context.Context) ([]boiler.Schedule, error) {
	lines, err := b.get(ctx, "/par.cgi")
	if err != nil {
		return nil, err
	}
	params := make(map[string][]int)
	for _, line := range lines {
		if id, ok, vals := parseParamInts(line); ok {
			params[id] = vals
		}
	}
	return buildSchedules(params), nil
}
