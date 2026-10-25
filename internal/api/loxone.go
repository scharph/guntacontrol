package api

import (
	"encoding/xml"
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
	mux.HandleFunc("/loxone/template", s.handleLoxoneTemplate)
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
	if s.settings.ReadOnly() {
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
	for _, k := range s.insightKPIs() {
		fmt.Fprintf(w, "%s=%s\n", k.Key, loxone.Value(k.Value))
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
	for _, k := range s.insightKPIs() {
		if strings.ToLower(k.Key) == want {
			fmt.Fprint(w, loxone.Value(k.Value))
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
	if !s.settings.AllowWrite() {
		http.Error(w, "ERR: Schreiben ist deaktiviert (allowWrite: false)", http.StatusForbidden)
		return
	}
	if s.settings.ReadOnly() {
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

// ---- Loxone templates (import in LOXONE Config via "Vorlage laden") ----

type vhiCmd struct {
	XMLName       xml.Name `xml:"VirtualInHttpCmd"`
	Title         string   `xml:"Title,attr"`
	Comment       string   `xml:"Comment,attr"`
	Check         string   `xml:"Check,attr"`
	Signed        bool     `xml:"Signed,attr"`
	Analog        bool     `xml:"Analog,attr"`
	SourceValLow  float64  `xml:"SourceValLow,attr"`
	DestValLow    float64  `xml:"DestValLow,attr"`
	SourceValHigh float64  `xml:"SourceValHigh,attr"`
	DestValHigh   float64  `xml:"DestValHigh,attr"`
	DefVal        float64  `xml:"DefVal,attr"`
	MinVal        float64  `xml:"MinVal,attr"`
	MaxVal        float64  `xml:"MaxVal,attr"`
}

type vhiTemplate struct {
	XMLName     xml.Name `xml:"VirtualInHttp"`
	Title       string   `xml:"Title,attr"`
	Comment     string   `xml:"Comment,attr"`
	Address     string   `xml:"Address,attr"`
	PollingTime int      `xml:"PollingTime,attr"`
	Cmds        []vhiCmd `xml:"VirtualInHttpCmd"`
}

type voCmd struct {
	XMLName      xml.Name `xml:"VirtualOutCmd"`
	Title        string   `xml:"Title,attr"`
	Comment      string   `xml:"Comment,attr"`
	CmdOnMethod  string   `xml:"CmdOnMethod,attr"`
	CmdOn        string   `xml:"CmdOn,attr"`
	CmdOffMethod string   `xml:"CmdOffMethod,attr"`
	CmdOff       string   `xml:"CmdOff,attr"`
	Analog       bool     `xml:"Analog,attr"`
}

type voTemplate struct {
	XMLName        xml.Name `xml:"VirtualOut"`
	Title          string   `xml:"Title,attr"`
	Comment        string   `xml:"Comment,attr"`
	Address        string   `xml:"Address,attr"`
	CmdInit        string   `xml:"CmdInit,attr"`
	CloseAfterSend bool     `xml:"CloseAfterSend,attr"`
	Cmds           []voCmd  `xml:"VirtualOutCmd"`
}

// handleLoxoneTemplate emits a ready-to-import LOXONE Config template:
//
//	GET /loxone/template          Virtual HTTP Input (reads /loxone/status) — monitoring
//	GET /loxone/template?type=out Virtual Output (writes /loxone/set) — circuit programs + hot-water reload
func (s *Server) handleLoxoneTemplate(w http.ResponseWriter, r *http.Request) {
	base := "http://" + r.Host
	var (
		payload  any
		fileName string
	)

	if strings.EqualFold(r.URL.Query().Get("type"), "out") {
		vo := voTemplate{Title: "guntacontrol", Address: base, CloseAfterSend: true}
		// One digital command per active heating circuit: ON = Normal (1), OFF = Absenken (3).
		active, _ := s.sched.ActiveCircuits(r.Context())
		names, _ := s.sched.CircuitNames(r.Context())
		for _, c := range active {
			if c == 1 {
				continue // HK1 ("Heizkörper") is intentionally excluded
			}
			name := names[c]
			if name == "" {
				name = fmt.Sprintf("Heizkreis %d", c)
			}
			syn := fmt.Sprintf("HK%d01", c)
			vo.Cmds = append(vo.Cmds, voCmd{
				Title:        name + " Heizen/Absenken",
				CmdOnMethod:  "GET",
				CmdOn:        fmt.Sprintf("/loxone/set?syn=%s&value=1", syn),
				CmdOffMethod: "GET",
				CmdOff:       fmt.Sprintf("/loxone/set?syn=%s&value=3", syn),
			})
		}
		// Pulse to request a one-off hot-water reload (BK006 = Ja); the boiler resets it.
		vo.Cmds = append(vo.Cmds, voCmd{
			Title:       "Warmwasser nachladen",
			CmdOnMethod: "GET",
			CmdOn:       "/loxone/set?syn=BK006&value=1",
		})
		payload, fileName = vo, "guntacontrol-ausgang.xml"
	} else {
		vhi := vhiTemplate{
			Title:       "guntacontrol",
			Comment:     "Guntamatic Biostar Flex",
			Address:     base + "/loxone/status",
			PollingTime: 30,
		}
		for _, v := range s.poller.Snapshot().Values {
			key := loxone.Key(v.Name)
			if key == "" {
				continue
			}
			vhi.Cmds = append(vhi.Cmds, vhiCmd{
				Title: v.Name, Check: key + "=", Signed: true, Analog: true,
				SourceValHigh: 100, DestValHigh: 100, MinVal: -100000, MaxVal: 100000,
			})
		}
		for _, k := range s.insightKPIs() {
			vhi.Cmds = append(vhi.Cmds, vhiCmd{
				Title: k.Title, Comment: "Auswertung", Check: k.Key + "=", Signed: true, Analog: true,
				SourceValHigh: 100, DestValHigh: 100, MinVal: -100000, MaxVal: 100000,
			})
		}
		payload, fileName = vhi, "guntacontrol-eingang.xml"
	}

	out, err := xml.MarshalIndent(payload, "", "\t")
	if err != nil {
		http.Error(w, "ERR: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	fmt.Fprint(w, xml.Header)
	w.Write(out)
}

func (s *Server) handleLoxoneHelp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	host := r.Host
	fmt.Fprintf(w, "guntacontrol – Loxone interface\n\n")
	fmt.Fprintf(w, "Templates for LOXONE Config (save to a file, then \"Vorlage laden\"):\n")
	fmt.Fprintf(w, "  http://%s/loxone/template            Virtueller HTTP Eingang (Monitoring)\n", host)
	fmt.Fprintf(w, "  http://%s/loxone/template?type=out   Virtueller Ausgang (Heizkreis-Steuerung)\n\n", host)
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
	if kpis := s.insightKPIs(); len(kpis) > 0 {
		fmt.Fprintf(w, "\nAuswertung keys (pellets estimated; full JSON at /api/insights):\n")
		for _, k := range kpis {
			unit := ""
			if k.Unit != "" {
				unit = " [" + k.Unit + "]"
			}
			fmt.Fprintf(w, "  %s=%s%s   (%s)\n", k.Key, loxone.Value(k.Value), unit, k.Title)
		}
	}
}
