// Package api exposes the polled boiler data over a small HTTP API, a Prometheus-style
// metrics endpoint and the embedded web UI, and accepts control writes.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"guntacontrol/internal/boiler"
	"guntacontrol/internal/service"
	"guntacontrol/internal/store"
)

// ParamWriter writes a parameter by synonym.
type ParamWriter interface {
	SetParam(ctx context.Context, syn, value string) (string, error)
}

// DeviceReader reads configuration the device exposes via par.cgi: the time programs
// ("Uhrenprogramme") and the per-circuit heating curves ("Heizkurve"). Both read-only.
type DeviceReader interface {
	Schedules(ctx context.Context) ([]boiler.Schedule, error)
	HeatCurves(ctx context.Context) ([]boiler.HeatCurve, error)
	Params(ctx context.Context) ([]boiler.Param, error)
	// CircuitNames returns user-set heating-circuit names (HKx11) by 0-based index.
	CircuitNames(ctx context.Context) (map[int]string, error)
	// ActiveCircuits returns the 0-based indices of configured circuits (HPx02 != Kein).
	ActiveCircuits(ctx context.Context) ([]int, error)
}

// SettingsStore exposes the write-protection settings, backed by the YAML config.
type SettingsStore interface {
	// AllowWrite is the master capability: when false, writes are impossible and the
	// read-only toggle can never be disabled.
	AllowWrite() bool
	// ReadOnly is the user-facing write protection ("Schreibschutz").
	ReadOnly() bool
	// SetReadOnly persists the write-protection toggle.
	SetReadOnly(on bool) error
	// OverviewCircuits lists the heating circuits (0-based) shown on the dashboard.
	OverviewCircuits() []int
}

// Server wires the poller, the SQLite-backed history/audit stores and the embedded UI
// into the HTTP surface.
type Server struct {
	poller   *service.Poller
	param    ParamWriter
	settings SettingsStore
	// webFS is the embedded single-page app served at / with history-API fallback.
	webFS   fs.FS
	history *store.History
	sched   DeviceReader
	audit   *store.AuditLog
}

// NewServer builds the HTTP server from its collaborators. history, sched, audit and
// webFS may be nil to disable the corresponding features.
func NewServer(p *service.Poller, param ParamWriter, sched DeviceReader, history *store.History, audit *store.AuditLog, webFS fs.FS, settings SettingsStore) *Server {
	return &Server{
		poller:   p,
		param:    param,
		sched:    sched,
		history:  history,
		audit:    audit,
		webFS:    webFS,
		settings: settings,
	}
}

// clientIP extracts the caller's address for the audit log, honouring a single
// X-Forwarded-For hop when behind a proxy.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// actor names the interface a write came through: an explicit X-Actor header or ?via=
// (set by the web app), else the given default.
func actor(r *http.Request, def string) string {
	if v := r.Header.Get("X-Actor"); v != "" {
		return v
	}
	if v := r.URL.Query().Get("via"); v != "" {
		return v
	}
	return def
}

// record writes an audit entry if auditing is enabled.
func (s *Server) record(e store.AuditEntry) {
	if s.audit != nil {
		s.audit.Record(e)
	}
}

func (s *Server) IsReadOnly() bool { return s.settings.ReadOnly() }

// setReadOnly updates the write protection and persists it to the config. Enabling is
// always allowed; disabling requires the master write capability.
func (s *Server) setReadOnly(on bool) error {
	if !on && !s.settings.AllowWrite() {
		return fmt.Errorf("Schreiben ist deaktiviert (allowWrite: false in der Konfiguration)")
	}
	return s.settings.SetReadOnly(on)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/values", s.handleValues)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/param", s.handleParam)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/insights", s.handleInsights)
	mux.HandleFunc("/api/schedules", s.handleSchedules)
	mux.HandleFunc("/api/heatcurve", s.handleHeatCurves)
	mux.HandleFunc("/api/params", s.handleParams)
	mux.HandleFunc("/api/audit", s.handleAudit)
	s.registerLoxone(mux)
	return mux
}

// handleHistory returns downsampled series for the Auswertung chart:
//
//	GET /api/history?channels=Aussentemperatur,Kesseltemperatur&from=<ms>&to=<ms>&max=500
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if s.history == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "history disabled"})
		return
	}
	q := r.URL.Query()
	to := time.Now()
	from := to.Add(-24 * time.Hour)
	if v := q.Get("from"); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			from = time.UnixMilli(ms)
		}
	}
	if v := q.Get("to"); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			to = time.UnixMilli(ms)
		}
	}
	channels := store.HistoryChannels
	if v := q.Get("channels"); v != "" {
		channels = strings.Split(v, ",")
	}
	maxPts := 500
	if v := q.Get("max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPts = n
		}
	}
	units := s.channelUnits()
	writeJSON(w, http.StatusOK, map[string]any{
		"from":   from.UnixMilli(),
		"to":     to.UnixMilli(),
		"series": s.history.Range(from, to, channels, maxPts, units),
	})
}

// handleInsights returns the per-day table and hints: GET /api/insights?days=14.
func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	if s.history == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "history disabled"})
		return
	}
	days := 14
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = n
		}
	}
	writeJSON(w, http.StatusOK, s.history.Insights(days))
}

// handleSchedules returns the decoded time programs ("Uhrenprogramme"), read-only.
func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	if s.sched == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Zeitprogramme nicht verfügbar"})
		return
	}
	schedules, err := s.sched.Schedules(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if f := s.circuitFilter(r.Context()); f != nil {
		kept := schedules[:0]
		for _, sc := range schedules {
			if sc.Circuit < 0 || f[sc.Circuit] { // keep non-heating (water/circulation) always
				kept = append(kept, sc)
			}
		}
		schedules = kept
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": schedules})
}

// handleHeatCurves returns the configured heating curves per circuit, read-only.
func (s *Server) handleHeatCurves(w http.ResponseWriter, r *http.Request) {
	if s.sched == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Heizkurve nicht verfügbar"})
		return
	}
	curves, err := s.sched.HeatCurves(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if f := s.circuitFilter(r.Context()); f != nil {
		kept := curves[:0]
		for _, c := range curves {
			if f[c.Circuit] {
				kept = append(kept, c)
			}
		}
		curves = kept
	}
	writeJSON(w, http.StatusOK, map[string]any{"curves": curves})
}

// handleParams returns the full par.cgi parameter table (values + limits), read-only.
func (s *Server) handleParams(w http.ResponseWriter, r *http.Request) {
	if s.sched == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Parameter nicht verfügbar"})
		return
	}
	params, err := s.sched.Params(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"params": params})
}

// handleAudit returns the recent change log (newest first): GET /api/audit?limit=100.
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if s.audit == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "audit disabled"})
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": s.audit.Recent(limit)})
}

// channelUnits maps channel name -> unit from the latest snapshot.
func (s *Server) channelUnits() map[string]string {
	m := make(map[string]string)
	for _, v := range s.poller.Snapshot().Values {
		m[v.Name] = v.Unit
	}
	return m
}

// valuesResponse is the /api/values body: the device snapshot plus the server-level
// write-protection state the UI needs to gate its controls.
type valuesResponse struct {
	boiler.Snapshot
	ReadOnly     bool           `json:"readOnly"`
	AllowWrite   bool           `json:"allowWrite"`
	Circuits     []int          `json:"circuits"`
	CircuitNames map[int]string `json:"circuitNames"`
}

func (s *Server) handleValues(w http.ResponseWriter, r *http.Request) {
	// Circuit metadata is cached in the device backend; ignore transient read errors and
	// fall back to the default titles / "show all" in the UI.
	names, _ := s.sched.CircuitNames(r.Context())
	writeJSON(w, http.StatusOK, valuesResponse{
		Snapshot:     s.poller.Snapshot(),
		ReadOnly:     s.settings.ReadOnly(),
		AllowWrite:   s.settings.AllowWrite(),
		Circuits:     s.effectiveCircuits(r.Context()),
		CircuitNames: names,
	})
}

// effectiveCircuits is the set of heating circuits to show: an explicit overview.circuits
// config wins; otherwise the device's active circuits (HPx02 != Kein). An empty result
// (e.g. a transient par.cgi error) is treated as "show all" by the callers.
func (s *Server) effectiveCircuits(ctx context.Context) []int {
	if cfg := s.settings.OverviewCircuits(); len(cfg) > 0 {
		return cfg
	}
	active, _ := s.sched.ActiveCircuits(ctx)
	return active
}

// circuitFilter returns a membership set of circuits to keep, or nil to keep all.
func (s *Server) circuitFilter(ctx context.Context) map[int]bool {
	eff := s.effectiveCircuits(ctx)
	if len(eff) == 0 {
		return nil
	}
	m := make(map[int]bool, len(eff))
	for _, c := range eff {
		m[c] = true
	}
	return m
}

// handleSettings reads or updates server settings.
//
//	GET  /api/settings             -> {"readOnly":bool,"allowWrite":bool}
//	POST /api/settings {readOnly}  -> same, after applying the change
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// nothing to do; fall through to the common response below
	case http.MethodPost:
		var body struct {
			ReadOnly *bool `json:"readOnly"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ReadOnly == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "need {readOnly: bool}"})
			return
		}
		err := s.setReadOnly(*body.ReadOnly)
		entry := store.AuditEntry{
			Who: clientIP(r), Via: actor(r, "api"), Action: "settings",
			Target: "readOnly", Value: strconv.FormatBool(*body.ReadOnly),
		}
		if err != nil {
			entry.Result, entry.Detail = "error", err.Error()
			s.record(entry)
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		entry.Result = "ok"
		s.record(entry)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET or POST"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"readOnly":   s.settings.ReadOnly(),
		"allowWrite": s.settings.AllowWrite(),
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	snap := s.poller.Snapshot()
	if snap.Err != "" || snap.Updated.IsZero() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "error": snap.Err, "updated": snap.Updated,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": snap.Updated})
}

// handleParam writes a parameter by synonym:
// POST /api/param  body {"syn":"PR001","value":"1"}.
func (s *Server) handleParam(w http.ResponseWriter, r *http.Request) {
	if !s.writeAllowed(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var body struct {
		Syn   string `json:"syn"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Syn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "need {syn, value}"})
		return
	}
	ack, err := s.param.SetParam(r.Context(), body.Syn, body.Value)
	entry := store.AuditEntry{
		Who: clientIP(r), Via: actor(r, "api"), Action: "param",
		Target: body.Syn, Value: body.Value,
	}
	if err != nil {
		entry.Result, entry.Detail = "error", err.Error()
		s.record(entry)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	entry.Result, entry.Detail = "ok", ack
	s.record(entry)
	writeJSON(w, http.StatusOK, map[string]string{"ack": ack})
}

func (s *Server) writeAllowed(w http.ResponseWriter) bool {
	if !s.settings.AllowWrite() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Schreiben ist deaktiviert (allowWrite: false in der Konfiguration)"})
		return false
	}
	if s.settings.ReadOnly() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Schreibschutz aktiv – in den Einstellungen deaktivieren"})
		return false
	}
	return true
}

var metricNameRe = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// handleMetrics renders numeric and bool channels in Prometheus text format.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	snap := s.poller.Snapshot()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	fmt.Fprintf(w, "# HELP guntamatic_up 1 if the last poll succeeded.\n")
	fmt.Fprintf(w, "# TYPE guntamatic_up gauge\n")
	up := 0
	if snap.Err == "" && !snap.Updated.IsZero() {
		up = 1
	}
	fmt.Fprintf(w, "guntamatic_up %d\n", up)

	for _, v := range snap.Values {
		var f float64
		switch x := v.Value.(type) {
		case float64:
			f = x
		case bool:
			if x {
				f = 1
			}
		default:
			continue // strings and undecodable values are skipped
		}
		metric := "guntamatic_" + sanitize(v.Name)
		fmt.Fprintf(w, "%s{id=\"%d\",unit=%q} %g\n", metric, v.ID, v.Unit, f)
	}
}

func sanitize(name string) string {
	s := metricNameRe.ReplaceAllString(strings.ToLower(name), "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "value"
	}
	return s
}

// handleIndex serves the embedded single-page app at "/", which catches every non-API
// path. The embedded UI is always present (wired in main); without it "/" is a 404.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if s.webFS == nil {
		http.NotFound(w, r)
		return
	}
	s.serveSPA(w, r)
}

// serveSPA serves static files from the embedded UI, falling back to index.html for
// paths that do not map to a real file. path.Clean plus fs.FS semantics reject traversal.
func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if f, err := s.webFS.Open(name); err == nil {
		fi, _ := f.Stat()
		f.Close()
		if fi != nil && !fi.IsDir() {
			http.ServeFileFS(w, r, s.webFS, name)
			return
		}
	}
	http.ServeFileFS(w, r, s.webFS, "index.html")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
