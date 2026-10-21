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
	Kind     string            `json:"kind"` // "heating" | "water" | "circulation"
	Variants []ScheduleVariant `json:"variants"`
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
