package store

import (
	"database/sql"
	"log"
	"time"
)

// AuditEntry is one recorded change attempt: when, who (client IP), through which
// interface, what was changed and the outcome.
type AuditEntry struct {
	Time   time.Time `json:"time"`
	Who    string    `json:"who"`    // client IP (X-Forwarded-For first hop, else RemoteAddr)
	Via    string    `json:"via"`    // web | loxone | api | modbus
	Action string    `json:"action"` // param | register | settings
	Target string    `json:"target"` // synonym / register address / "readOnly"
	Value  string    `json:"value"`  // requested value
	Result string    `json:"result"` // ok | error
	Detail string    `json:"detail,omitempty"`
}

// AuditLog records every parameter/setting change to the shared SQLite database
// (table "audit") and serves the recent tail via /api/audit.
type AuditLog struct {
	db *sql.DB
}

func NewAuditLog(db *sql.DB) *AuditLog {
	return &AuditLog{db: db}
}

// Record stores an entry in SQLite and mirrors it to the process log.
func (a *AuditLog) Record(e AuditEntry) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	log.Printf("audit: %s via=%s %s %s=%s -> %s %s",
		e.Who, e.Via, e.Action, e.Target, e.Value, e.Result, e.Detail)

	if a.db == nil {
		return
	}
	_, err := a.db.Exec(
		`INSERT INTO audit(ts, who, via, action, target, value, result, detail)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Time.UnixMilli(), e.Who, e.Via, e.Action, e.Target, e.Value, e.Result, e.Detail)
	if err != nil {
		log.Printf("audit: insert failed: %v", err)
	}
}

// Recent returns up to limit most-recent entries, newest first.
func (a *AuditLog) Recent(limit int) []AuditEntry {
	if a.db == nil {
		return nil
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := a.db.Query(
		`SELECT ts, who, via, action, target, value, result, detail
		 FROM audit ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		log.Printf("audit: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	out := make([]AuditEntry, 0, limit)
	for rows.Next() {
		var ms int64
		var e AuditEntry
		if err := rows.Scan(&ms, &e.Who, &e.Via, &e.Action, &e.Target, &e.Value, &e.Result, &e.Detail); err != nil {
			log.Printf("audit: scan failed: %v", err)
			return out
		}
		e.Time = time.UnixMilli(ms)
		out = append(out, e)
	}
	return out
}
