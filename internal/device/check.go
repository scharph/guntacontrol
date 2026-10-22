package device

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"guntacontrol/internal/boiler"
)

// ConfigCheck reads par.cgi and checks the boiler configuration against the live values
// for common operating mistakes (unused circuits left active, missing heating windows,
// circuits running at their flow limit, …). Read-only; it only reports.
func (b *FlexBackend) ConfigCheck(ctx context.Context, live []boiler.Value) ([]boiler.Finding, error) {
	lines, err := b.get(ctx, "/par.cgi")
	if err != nil {
		return nil, err
	}
	return checkConfig(lines, live), nil
}

// Circuit program (HKx01) and drive mode (HPx02) enum indices.
const (
	progAus    = 0
	progNormal = 1

	driveKein    = 0
	drivePumpe   = 1
	driveMischer = 2 // 3 = Mischer (ECO)
)

// unconnected reports the placeholder readings of a sensor that is not wired.
func unconnected(v float64) bool { return v == 120 || v == -20 || v == -100 }

// checkConfig is the pure rule set behind ConfigCheck.
func checkConfig(lines []string, live []boiler.Value) []boiler.Finding {
	p := parFloats(lines)
	clocks := make(map[string][]int)
	for _, line := range lines {
		if id, ok, vals := parseParamInts(line); ok && strings.HasPrefix(line, id+";9;") {
			clocks[id] = vals
		}
	}
	names := parseCircuitNames(lines)

	num := make(map[string]float64)
	for _, v := range live {
		if f, ok := v.Value.(float64); ok {
			num[v.Name] = f
		}
	}
	outdoor, haveOutdoor := num["Aussentemperatur"]

	var out []boiler.Finding
	add := func(level, title, detail string, syns ...string) {
		out = append(out, boiler.Finding{Level: level, Title: title, Detail: detail, Params: syns})
	}

	for _, c := range parseActiveCircuits(lines) {
		d := strconv.Itoa(c)
		syn := func(prefix, suffix string) string { return prefix + d + suffix }
		label := names[c]
		if label == "" {
			label = "Heizkreis " + d
		}
		drive := int(p[syn("HP", "02")])
		prog := int(p[syn("HK", "01")])
		flowMin, flowMax := p[syn("HP", "05")], p[syn("HP", "06")]
		flow, haveFlow := num["Vorlauf Ist "+d]

		// A mixer circuit cannot regulate without its flow sensor.
		if drive >= driveMischer && (!haveFlow || unconnected(flow)) {
			add("warn", label+": Mischerkreis ohne Vorlauffühler",
				fmt.Sprintf("Vorlauf Ist %s liest %s – der Fühler ist nicht angeschlossen. Wird der Kreis nicht genutzt, Betrieb HK%s auf „Kein“ oder das Programm auf AUS stellen.", d, fmtTemp(flow, haveFlow), d),
				syn("HP", "02"), syn("HK", "01"))
			continue
		}

		// Active but unnamed while other circuits carry names: likely a leftover.
		if names[c] == "" && len(names) > 0 && prog != progAus {
			kind := "Mischerkreis"
			if drive == drivePumpe {
				kind = "Pumpenkreis"
			}
			add("warn", label+": aktiv, aber ohne Namen",
				fmt.Sprintf("Als %s konfiguriert, Programm nicht AUS. Ist hier wirklich ein Kreis angeschlossen? Ein ungenutzter aktiver Kreis meldet Wärmebedarf an (Vorlauf min %.0f °C) und kann den Puffer unnötig hoch halten.", kind, flowMin),
				syn("HP", "02"), syn("HK", "01"))
		}

		// Heating windows of the time program that NORMAL follows.
		if prog == progNormal {
			for _, v := range clockVariants(c, int(p[syn("HK", "02a")])) {
				vals, ok := clocks[v.id]
				if !ok {
					continue
				}
				var empty []string
				for _, day := range decodeClockProgram(vals) {
					if len(day.Windows) == 0 {
						empty = append(empty, day.Day)
					}
				}
				switch {
				case len(empty) == len(weekdays):
					add("warn", label+": Programm NORMAL ohne Heizzeiten",
						fmt.Sprintf("Das Uhrenprogramm (%s) ist leer – der Kreis läuft dauerhaft abgesenkt (Nacht %.1f °C). Wird er selten genutzt, besser auf AUS stellen und bei Bedarf auf HEIZEN.", v.label, p[syn("HK", "05")]),
						syn("HK", "01"), v.id)
				case len(empty) > 0:
					add("warn", label+": Tag ohne Heizfenster",
						fmt.Sprintf("Im Uhrenprogramm (%s) hat %s kein Heizfenster – der Kreis bleibt dort den ganzen Tag abgesenkt. Tippfehler?", v.label, strings.Join(empty, ", ")),
						v.id)
				}
			}
		}

		if drive < driveMischer {
			continue // pump circuits have no own flow control; the rest is mixer-only
		}

		// Running at the configured maximum although it is not cold yet.
		if haveFlow && haveOutdoor && outdoor > 5 && flowMax > 0 && flow >= flowMax-1 {
			add("warn", label+": Vorlauf am Maximum",
				fmt.Sprintf("Vorlauf %.1f °C bei max. %.0f °C schon bei %.1f °C außen – bei Frost wird der Kreis gekappt. Wird es zu kalt, Vorlauf max anheben (Estrich-Grenze beachten), nicht die Steilheit.", flow, flowMax, outdoor),
				syn("HP", "06"))
		}

		// Floor heating (low flow limit): heating limit and night setback.
		if flowMax > 0 && flowMax <= 45 {
			if limit := p[syn("HK", "09")]; limit > 16 {
				add("info", label+": Heizgrenze hoch für Fußbodenheizung",
					fmt.Sprintf("AT-Abschaltung %.0f °C. Für Flächenheizung in einem gedämmten Haus reichen meist 15–16 °C – spart in der Übergangszeit Pellets.", limit),
					syn("HK", "09"))
			}
			if day, night := p[syn("HK", "04")], p[syn("HK", "05")]; day-night > 3 {
				add("info", label+": große Nachtabsenkung",
					fmt.Sprintf("Tag %.1f °C / Nacht %.1f °C (−%.1f K). Fußbodenheizung ist träge – das Wiederaufheizen frisst den Großteil der Ersparnis; 2–3 K sind sinnvoller.", day, night, day-night),
					syn("HK", "05"))
			}
		}
	}

	// Domestic hot water.
	if soll, ok := p["BK004"]; ok {
		if soll > 60 {
			add("info", "Warmwasser-Soll über 60 °C",
				fmt.Sprintf("Soll %.0f °C. Höhere Speichertemperatur heißt mehr Verluste; 55–60 °C reichen meist.", soll), "BK004")
		}
		if ist, ok := num["Warmwasser 0"]; ok && !unconnected(ist) && ist >= soll+5 {
			add("info", "Warmwasser deutlich über Soll",
				fmt.Sprintf("Gemessen %.1f °C bei Soll %.0f °C – der Speicher wird überladen (z. B. aus dem heißen Puffer). Puffertemperaturen prüfen.", ist, soll), "BK004")
		}
	}

	if h, ok := num["Asche leeren in"]; ok && h < 24 {
		add("warn", "Asche bald leeren", fmt.Sprintf("Noch %.0f Betriebsstunden bis zur Aschewarnung.", h))
	}

	if len(out) == 0 {
		add("info", "Konfiguration unauffällig", "Keine der geprüften Regeln hat angeschlagen.")
	}
	return out
}

// clockVariants returns the time programs a circuit in NORMAL follows, by its weather
// setting HKx02a (0=AUTO uses all three by outdoor temperature, 1=kalt, 2=mild, 3=warm).
func clockVariants(c, weather int) []variantSpec {
	if c < 0 || c >= len(scheduleSpecs) || scheduleSpecs[c].circuit != c {
		return nil
	}
	all := scheduleSpecs[c].variants
	if weather >= 1 && weather <= len(all) {
		return all[weather-1 : weather]
	}
	return all
}

func fmtTemp(v float64, ok bool) string {
	if !ok {
		return "nichts"
	}
	return fmt.Sprintf("%.0f °C", v)
}
