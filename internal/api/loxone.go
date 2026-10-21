package api

import (
	"fmt"
	"net/http"
	"strings"

	"guntacontrol/internal/loxone"
	"guntacontrol/internal/store"
)

// Loxone integration: a plain-text HTTP surface for the Loxone Miniserver (it does not
// do JSON/POST easily).
//
//	GET /loxone/status            -> text/plain, one "Key=Value" line per channel + meta
//	GET /loxone/get?name=<key>    -> the bare value of one channel (e.g. "72.4")
//	GET /loxone/set?syn=<S>&value=<V> -> "OK" or "ERR: ..." (GET-based parameter write)
//	GET /loxone                   -> short help listing the live keys

func (s *Server) registerLoxone(mux *http.ServeMux) {
	mux.HandleFunc("/loxone", s.handleLoxoneHelp)
	mux.HandleFunc("/loxone/status", s.handleLoxoneStatus)
	mux.HandleFunc("/loxone/get", s.handleLoxoneGet)
	mux.HandleFunc("/loxone/set", s.handleLoxoneSet)
}

func (s *Server) handleLoxoneStatus(w http.ResponseWriter, r *http.Request) {
	snap := s.poller.Snapshot()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	online := 0
	if snap.Err == "" && !snap.Updated.IsZero() {
		online = 1
	}
	fmt.Fprintf(w, "online=%d\n", online)
	fmt.Fprintf(w, "updated=%d\n", snap.Updated.Unix())
	ro := 0
	if s.readOnly.Load() {
		ro = 1
	}
	fmt.Fprintf(w, "readonly=%d\n", ro)

	for _, v := range snap.Values {
		key := loxone.Key(v.Name)
		if key == "" {
			continue
		}
		fmt.Fprintf(w, "%s=%s\n", key, loxone.Value(v.Value))
	}
}

func (s *Server) handleLoxoneGet(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if name == "" {
		http.Error(w, "ERR: need ?name=<channel key or name>", http.StatusBadRequest)
		return
	}
	want := strings.ToLower(name)
	for _, v := range s.poller.Snapshot().Values {
		if strings.EqualFold(v.Name, name) || strings.ToLower(loxone.Key(v.Name)) == want {
			fmt.Fprint(w, loxone.Value(v.Value))
			return
		}
	}
	http.Error(w, "ERR: unknown channel "+name, http.StatusNotFound)
}

func (s *Server) handleLoxoneSet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	syn := strings.TrimSpace(r.URL.Query().Get("syn"))
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	if syn == "" || value == "" {
		http.Error(w, "ERR: need ?syn=<synonym>&value=<v>", http.StatusBadRequest)
		return
	}
	if !s.allowWrite {
		http.Error(w, "ERR: writes disabled; start with -allow-write", http.StatusForbidden)
		return
	}
	if s.readOnly.Load() {
		http.Error(w, "ERR: Schreibschutz aktiv", http.StatusForbidden)
		return
	}
	if s.param == nil {
		http.Error(w, "ERR: no parameter writer configured", http.StatusBadRequest)
		return
	}
	ack, err := s.param.SetParam(r.Context(), syn, value)
	entry := store.AuditEntry{
		Who: clientIP(r), Via: "loxone", Action: "param", Target: syn, Value: value,
	}
	if err != nil {
		entry.Result, entry.Detail = "error", err.Error()
		s.record(entry)
		http.Error(w, "ERR: "+err.Error(), http.StatusBadGateway)
		return
	}
	entry.Result, entry.Detail = "ok", ack
	s.record(entry)
	if ack == "" {
		ack = "OK"
	}
	fmt.Fprint(w, ack)
}

func (s *Server) handleLoxoneHelp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	host := r.Host
	fmt.Fprintf(w, "guntacontrol – Loxone interface\n\n")
	fmt.Fprintf(w, "Read (Virtueller HTTP Eingang):\n")
	fmt.Fprintf(w, "  http://%s/loxone/status            all channels as Key=Value\n", host)
	fmt.Fprintf(w, "  http://%s/loxone/get?name=<key>    one bare value\n\n", host)
	fmt.Fprintf(w, "Write (Virtueller Ausgang, value via <v>):\n")
	fmt.Fprintf(w, "  http://%s/loxone/set?syn=PR001&value=<v>   Programm (0=AUS 1=NORMAL 3=HEIZEN ...)\n", host)
	fmt.Fprintf(w, "  http://%s/loxone/set?syn=HK001&value=<v>   Heizkreis 0 (0=AUS 1=NORMAL 2=HEIZEN 3=ABSENKEN)\n\n", host)
	fmt.Fprintf(w, "Live channel keys (copy as Loxone start marker, incl. the '='):\n")
	for _, v := range s.poller.Snapshot().Values {
		key := loxone.Key(v.Name)
		if key == "" {
			continue
		}
		unit := ""
		if v.Unit != "" {
			unit = " [" + v.Unit + "]"
		}
		fmt.Fprintf(w, "  %s=%s%s   (%s)\n", key, loxone.Value(v.Value), unit, v.Name)
	}
}
