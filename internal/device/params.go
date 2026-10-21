package device

import (
	"context"
	"strconv"
	"strings"

	"guntacontrol/internal/boiler"
)

// Params fetches par.cgi and returns the decoded parameter table (all configurable
// points with their current value and, for numeric parameters, the allowed range).
// Read-only.
func (b *FlexBackend) Params(ctx context.Context) ([]boiler.Param, error) {
	lines, err := b.get(ctx, "/par.cgi")
	if err != nil {
		return nil, err
	}
	out := make([]boiler.Param, 0, len(lines))
	for _, line := range lines {
		if p, ok := parseParam(line); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// par.cgi line layout by type field:
//
//	2/3  numeric  SYN;t;value;min;max;step;unit;default;0;0;0;Label;
//	6    enum     SYN;t;index;…;Label;opt0;opt1;…
//	8    text     SYN;t;value;charset;Label;
//	9    clock     the week programs — shown on the Zeiten page, skipped here
func parseParam(line string) (boiler.Param, bool) {
	f := strings.Split(line, ";")
	if len(f) < 3 {
		return boiler.Param{}, false
	}
	syn := strings.TrimSpace(f[0])
	t, err := strconv.Atoi(strings.TrimSpace(f[1]))
	if err != nil || syn == "" {
		return boiler.Param{}, false // header / malformed line
	}

	p := boiler.Param{Syn: syn, Group: synPrefix(syn)}
	val := field(f, 2)

	switch t {
	case 2, 3: // numeric
		p.Kind = "number"
		p.Value = val
		p.Min = field(f, 3)
		p.Max = field(f, 4)
		p.Unit = field(f, 6)
		p.Default = field(f, 7)
		p.Label = lastNonEmpty(f)
	case 6: // enum: map the selected index to its option label
		p.Kind = "enum"
		p.Label = field(f, 8)
		opts := []string{}
		if len(f) > 9 {
			opts = f[9:]
		}
		if idx, e := strconv.Atoi(val); e == nil && idx >= 0 && idx < len(opts) {
			p.Value = strings.TrimSpace(opts[idx])
		} else {
			p.Value = val
		}
	case 8: // text
		p.Kind = "text"
		p.Value = val
		p.Label = lastNonEmpty(f)
	case 9: // clock program — handled on the Zeiten page
		return boiler.Param{}, false
	default: // 7 and others: value + label, no reliable limits
		p.Kind = "number"
		p.Value = val
		p.Unit = field(f, 6)
		p.Label = lastNonEmpty(f)
	}
	if p.Label == "" {
		p.Label = syn
	}
	return p, true
}

func field(f []string, i int) string {
	if i < len(f) {
		return strings.TrimSpace(f[i])
	}
	return ""
}

func lastNonEmpty(f []string) string {
	for i := len(f) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(f[i]); s != "" {
			return s
		}
	}
	return ""
}

// parseCircuitNames reads the user-set heating-circuit names from par.cgi. Each circuit
// has a text parameter HK<c>11 ("Benennung HK c"); only non-empty names are returned,
// keyed by the 0-based circuit index.
func parseCircuitNames(lines []string) map[int]string {
	m := make(map[int]string)
	for _, line := range lines {
		f := strings.Split(line, ";")
		if len(f) < 3 {
			continue
		}
		syn := strings.TrimSpace(f[0])
		// Benennung params are text (type 8) with synonym HK<digits>11.
		if len(syn) < 5 || syn[:2] != "HK" || strings.TrimSpace(f[1]) != "8" {
			continue
		}
		rest := syn[2:]
		if !strings.HasSuffix(rest, "11") {
			continue
		}
		c, err := strconv.Atoi(rest[:len(rest)-2])
		if err != nil {
			continue
		}
		if name := strings.TrimSpace(f[2]); name != "" {
			m[c] = name
		}
	}
	return m
}

// parseActiveCircuits returns the 0-based indices of configured heating circuits. A
// circuit is active when its drive mode HP<c>02 ("Betrieb HK c") is not 0 ("Kein");
// values 1/2/3 mean Pumpe/Mischer/Mischer (ECO).
func parseActiveCircuits(lines []string) []int {
	var out []int
	for _, line := range lines {
		f := strings.Split(line, ";")
		if len(f) < 3 {
			continue
		}
		syn := strings.TrimSpace(f[0])
		if len(syn) < 5 || syn[:2] != "HP" || strings.TrimSpace(f[1]) != "6" {
			continue
		}
		rest := syn[2:]
		if !strings.HasSuffix(rest, "02") {
			continue
		}
		c, err := strconv.Atoi(rest[:len(rest)-2])
		if err != nil {
			continue
		}
		if strings.TrimSpace(f[2]) != "0" {
			out = append(out, c)
		}
	}
	return out
}

// synPrefix returns the leading letters of a synonym (HK008 -> "HK"), used to group
// the parameter table by subsystem.
func synPrefix(s string) string {
	for i, r := range s {
		if r >= '0' && r <= '9' {
			return s[:i]
		}
	}
	return s
}
