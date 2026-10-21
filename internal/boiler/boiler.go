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
