// Package boiler holds the core data types shared across guntacontrol: live channel
// values, snapshots and decoded time programs. It has no dependencies so every other
// package can import it without cycles.
package boiler

import "time"

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

// ---- time programs (Uhrenprogramme) ----

// TimeWindow is one active interval of a day, in minutes since midnight.
type TimeWindow struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// DaySchedule is the list of active windows for one weekday.
type DaySchedule struct {
	Day     string       `json:"day"`
	Windows []TimeWindow `json:"windows"`
}

// ScheduleVariant is one time program (one par.cgi synonym), e.g. the "mild" variant.
type ScheduleVariant struct {
	ID    string        `json:"id"`
	Label string        `json:"label"`
	Days  []DaySchedule `json:"days"`
}

// Schedule groups the variants that belong to one circuit / hot-water program.
type Schedule struct {
	Title    string            `json:"title"`
	Kind     string            `json:"kind"`    // "heating" | "water"
	Circuit  int               `json:"circuit"` // 0-based heating circuit, -1 for non-heating
	Variants []ScheduleVariant `json:"variants"`
}

// ---- parameter table (par.cgi) ----

// Param is one configurable parameter from par.cgi: its current value plus, for numeric
// parameters, the allowed range. Kind drives how the UI renders it.
type Param struct {
	Syn     string `json:"syn"`
	Label   string `json:"label"`
	Kind    string `json:"kind"` // "number" | "enum" | "text"
	Value   string `json:"value"`
	Unit    string `json:"unit,omitempty"`
	Min     string `json:"min,omitempty"`
	Max     string `json:"max,omitempty"`
	Default string `json:"default,omitempty"`
	Group   string `json:"group"` // category derived from the synonym prefix
}

// ---- configuration check ----

// Finding is one observation of the configuration check: a setting that looks wrong or
// wasteful for the measured operation, with the par.cgi synonyms it refers to.
type Finding struct {
	Level  string   `json:"level"` // info | warn
	Title  string   `json:"title"`
	Detail string   `json:"detail"`
	Params []string `json:"params,omitempty"`
}

// ---- heating curve (Heizkurve) ----

// HeatCurve is the configured weather-compensation curve of one heating circuit,
// read from par.cgi. The Flex uses a slope ("Steilheit") model with a parallel shift
// and min/max flow-temperature limits; there is no published flow-setpoint formula, so
// the UI pairs these settings with the recorded outdoor/flow data rather than drawing a
// computed setpoint line.
type HeatCurve struct {
	Circuit        int     `json:"circuit"`        // 0-based heating circuit index
	Title          string  `json:"title"`          // "Heizkreis 0"
	Slope          float64 `json:"slope"`          // Steilheit (HKx07)
	Shift          float64 `json:"shift"`          // Parallelverschiebung, K (HPx09)
	FlowMin        float64 `json:"flowMin"`        // Vorlauf min, °C (HPx05)
	FlowMax        float64 `json:"flowMax"`        // Vorlauf max, °C (HPx06)
	OutdoorChannel string  `json:"outdoorChannel"` // daqdata channel for the outdoor temp
	FlowChannel    string  `json:"flowChannel"`    // daqdata channel for the actual flow temp
}

// CircuitSetpoint holds a heating circuit's room setpoints from par.cgi. The program
// selects which one applies: HEIZEN uses Day, ABSENKEN uses Night, NORMAL follows the
// clock program. The Level-1 key cannot write these; they are set on the boiler display.
type CircuitSetpoint struct {
	Day   float64 `json:"day"`   // Solltemp. Tag, °C (HKx04)
	Night float64 `json:"night"` // Solltemp. Nacht, °C (HKx05)
}

// DHW is the state of one domestic-hot-water tank (Warmwasser) from par.cgi.
type DHW struct {
	Setpoint float64 `json:"setpoint"` // WW Solltemp., °C (BKx04) — display only
	Reload   bool    `json:"reload"`   // WW Nachladen requested (BKx06 = Ja)
}
