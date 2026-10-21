package device

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"guntacontrol/internal/boiler"
)

// Heating-curve settings per circuit live in par.cgi:
//
//	HKx07  Steilheit (slope, 0.1-3.5)
//	HPx09  Parallelverschiebung (parallel shift, K)
//	HPx05  Vorlauft. min (°C)
//	HPx06  Vorlauft. max (°C)
//
// where x is the 0-based circuit index (HK007/HP009… for circuit 0, HK107/HP109… for
// circuit 1, and so on). We surface the first three circuits, matching the dashboard.

type curveSpec struct {
	circuit        int
	title          string
	outdoorChannel string
	flowChannel    string
}

var curveSpecs = []curveSpec{
	{0, "Heizkreis 0", "Aussentemperatur", "Vorlauf Ist 0"},
	{1, "Heizkreis 1", "Aussentemperatur", "Vorlauf Ist 1"},
	{2, "Heizkreis 2", "Aussentemperatur", "Vorlauf Ist 2"},
	{3, "Heizkreis 3", "Aussentemperatur", "Vorlauf Ist 3"},
	{4, "Heizkreis 4", "Aussentemperatur", "Vorlauf Ist 4"},
}

// parFloats parses par.cgi into a syn -> value map. The value is the third field of a
// line (SYN;type;value;min;max;step;unit;default;…;label;); non-numeric lines are skipped.
func parFloats(lines []string) map[string]float64 {
	m := make(map[string]float64, 512)
	for _, line := range lines {
		f := strings.Split(line, ";")
		if len(f) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(f[2]), 64)
		if err != nil {
			continue
		}
		m[strings.TrimSpace(f[0])] = v
	}
	return m
}

// HeatCurves fetches par.cgi and returns the configured heating curve of each circuit.
// It is read-only.
func (b *FlexBackend) HeatCurves(ctx context.Context) ([]boiler.HeatCurve, error) {
	lines, err := b.get(ctx, "/par.cgi")
	if err != nil {
		return nil, err
	}
	p := parFloats(lines)
	names := parseCircuitNames(lines)

	var out []boiler.HeatCurve
	for _, s := range curveSpecs {
		d := strconv.Itoa(s.circuit)
		slope, ok := p["HK"+d+"07"]
		if !ok {
			continue // circuit not present on this device
		}
		title := s.title
		if n := names[s.circuit]; n != "" {
			title = n
		}
		out = append(out, boiler.HeatCurve{
			Circuit:        s.circuit,
			Title:          title,
			Slope:          slope,
			Shift:          p[fmt.Sprintf("HP%s09", d)],
			FlowMin:        p[fmt.Sprintf("HP%s05", d)],
			FlowMax:        p[fmt.Sprintf("HP%s06", d)],
			OutdoorChannel: s.outdoorChannel,
			FlowChannel:    s.flowChannel,
		})
	}
	return out, nil
}
