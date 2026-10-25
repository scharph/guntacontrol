// Package device talks to the Guntamatic Biostar Flex over its modern local CSV API.
package device

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"guntacontrol/internal/boiler"
)

// FlexBackend implements the data source over the modern Guntamatic firmware's local
// CSV API (confirmed on a Biostar Flex, Software 3.3d):
//
//	GET /daqdesc.cgi   ->  one "Name;Unit" line per channel (ISO-8859-1)
//	GET /daqdata.cgi   ->  one value line per channel, same order
//
// Reading needs no key on the local network; the device's Sicherheitsschlüssel is only
// required for control/write requests. The key is still sent when configured, in case a
// firmware variant gates reads.
type FlexBackend struct {
	baseURL  string
	key      string // read key; not required by this firmware (reads are open)
	writeKey string // write key, required by /ext/parset.cgi for control
	http     *http.Client

	names []string // channel names, cached from daqdesc
	units []string

	// reqMu serializes all HTTP requests to the boiler. The Flex handles concurrent
	// requests poorly (a par.cgi read racing the data poll can fail), so every request
	// is sent one at a time.
	reqMu sync.Mutex

	// circuit metadata (names HKx11 + active modes HPx02) cached from par.cgi on a TTL.
	infoMu sync.Mutex
	info   *circuitInfo
	infoAt time.Time
}

// circuitInfo bundles the heating-circuit metadata read from one par.cgi fetch.
type circuitInfo struct {
	names     map[int]string                 // HKx11 custom names (non-empty only)
	active    []int                          // 0-based indices with HPx02 != "Kein"
	programs  map[int]int                    // HKx01 current program per circuit
	setpoints map[int]boiler.CircuitSetpoint // HKx04/HKx05 room setpoints per circuit
}

// circuitInfoTTL is how long cached circuit metadata is reused before par.cgi is read
// again (it changes rarely, so this keeps /api/values cheap).
const circuitInfoTTL = 10 * time.Minute

// circuitInfoCached returns the cached circuit metadata, refreshing it from par.cgi when
// stale. On a fetch error the last known value (possibly nil) is returned with the error.
func (b *FlexBackend) circuitInfoCached(ctx context.Context) (*circuitInfo, error) {
	b.infoMu.Lock()
	if b.info != nil && time.Since(b.infoAt) < circuitInfoTTL {
		ci := b.info
		b.infoMu.Unlock()
		return ci, nil
	}
	b.infoMu.Unlock()

	lines, err := b.get(ctx, "/par.cgi")
	if err != nil {
		b.infoMu.Lock()
		ci := b.info
		b.infoMu.Unlock()
		return ci, err
	}
	ci := &circuitInfo{
		names:     parseCircuitNames(lines),
		active:    parseActiveCircuits(lines),
		programs:  parseCircuitPrograms(lines),
		setpoints: parseCircuitSetpoints(lines),
	}
	b.infoMu.Lock()
	b.info = ci
	b.infoAt = time.Now()
	b.infoMu.Unlock()
	return ci, nil
}

// invalidateCircuitInfo drops the cached circuit metadata so the next read re-fetches
// par.cgi. Called after a successful write so the UI reflects the change immediately.
func (b *FlexBackend) invalidateCircuitInfo() {
	b.infoMu.Lock()
	b.info = nil
	b.infoMu.Unlock()
}

// CircuitPrograms returns the current program per circuit (HKx01), keyed by 0-based index.
func (b *FlexBackend) CircuitPrograms(ctx context.Context) (map[int]int, error) {
	ci, err := b.circuitInfoCached(ctx)
	if ci == nil {
		return nil, err
	}
	return ci.programs, err
}

// CircuitSetpoints returns the room setpoints (HKx04 Tag / HKx05 Nacht) per circuit.
func (b *FlexBackend) CircuitSetpoints(ctx context.Context) (map[int]boiler.CircuitSetpoint, error) {
	ci, err := b.circuitInfoCached(ctx)
	if ci == nil {
		return nil, err
	}
	return ci.setpoints, err
}

// CircuitNames returns the user-set heating-circuit names (HKx11), keyed by 0-based index.
func (b *FlexBackend) CircuitNames(ctx context.Context) (map[int]string, error) {
	ci, err := b.circuitInfoCached(ctx)
	if ci == nil {
		return nil, err
	}
	return ci.names, err
}

// ActiveCircuits returns the 0-based indices of configured heating circuits (HPx02 != Kein).
func (b *FlexBackend) ActiveCircuits(ctx context.Context) ([]int, error) {
	ci, err := b.circuitInfoCached(ctx)
	if ci == nil {
		return nil, err
	}
	return ci.active, err
}

// apiError is the device's JSON error/ack envelope, e.g. {"err":"Keine Berechtigung"}
// or {"ack":"..."} from /ext/parset.cgi.
type apiError struct {
	Err string `json:"err"`
	Ack string `json:"ack"`
}

func NewFlexBackend(host, key, writeKey string, timeout time.Duration) *FlexBackend {
	return &FlexBackend{
		baseURL:  "http://" + host,
		key:      key,
		writeKey: writeKey,
		http:     &http.Client{Timeout: timeout},
	}
}

// SetParam writes a control parameter via /ext/parset.cgi, using the synonyms from the
// boiler's own par.cgi table (e.g. PR001 Programm, HK001/HK101/HK201 Heizkreise).
// Control requires the write key; reads do not. Returns the device's ack text.
func (b *FlexBackend) SetParam(ctx context.Context, syn, value string) (string, error) {
	if b.writeKey == "" {
		return "", fmt.Errorf("kein Write-Key gesetzt (device.writeKey in der Konfiguration)")
	}
	q := url.Values{"key": {b.writeKey}, "syn": {syn}, "value": {value}}
	u := b.baseURL + "/ext/parset.cgi?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	b.reqMu.Lock()
	defer b.reqMu.Unlock()
	resp, err := b.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("parset: status %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var e apiError
	if err := json.Unmarshal(bytes.TrimSpace(raw), &e); err != nil {
		return "", fmt.Errorf("parset: decode %q: %w", string(raw), err)
	}
	if e.Err != "" {
		return "", fmt.Errorf("parset: device error: %s", e.Err)
	}
	// The write may have changed a cached par.cgi value (e.g. a circuit program), so
	// drop the cache to reflect it on the next read.
	b.invalidateCircuitInfo()
	return e.Ack, nil
}

func (b *FlexBackend) get(ctx context.Context, path string) ([]string, error) {
	u := b.baseURL + path
	if b.key != "" {
		u += "?" + url.Values{"key": {b.key}}.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	// Serialize device access (held through the body read) so this request does not
	// race the data poll.
	b.reqMu.Lock()
	defer b.reqMu.Unlock()
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %s", path, resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// The payload is ISO-8859-1 (e.g. "°C", "ä"); decode to UTF-8 by mapping each
	// byte to the identical Unicode code point.
	lines := make([]string, 0, 128)
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		lines = append(lines, latin1(sc.Bytes()))
	}
	return lines, sc.Err()
}

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

func (b *FlexBackend) ensureDesc(ctx context.Context) error {
	if b.names != nil {
		return nil
	}
	desc, err := b.get(ctx, "/daqdesc.cgi")
	if err != nil {
		return err
	}
	names := make([]string, len(desc))
	units := make([]string, len(desc))
	for i, line := range desc {
		name, unit, _ := strings.Cut(line, ";")
		names[i] = strings.TrimSpace(name)
		units[i] = strings.TrimSpace(unit)
	}
	b.names, b.units = names, units
	return nil
}

// Poll fetches the current channel values.
func (b *FlexBackend) Poll(ctx context.Context) ([]boiler.Value, error) {
	if err := b.ensureDesc(ctx); err != nil {
		return nil, err
	}
	data, err := b.get(ctx, "/daqdata.cgi")
	if err != nil {
		return nil, err
	}
	out := make([]boiler.Value, 0, len(b.names))
	for i, name := range b.names {
		if name == "" || strings.EqualFold(name, "reserved") {
			continue // skip padding/reserved channels
		}
		v := boiler.Value{ID: i, Name: name, Unit: b.units[i]}
		if i < len(data) {
			v.Type, v.Value = classify(strings.TrimSpace(data[i]))
		}
		out = append(out, v)
	}
	return out, nil
}

// classify infers a type and typed value from a raw CSV field. The Flex API encodes
// states as "EIN"/"AUS" text, numbers as decimal strings, and identifiers as text.
func classify(s string) (string, any) {
	switch strings.ToUpper(s) {
	case "":
		return "string", ""
	case "EIN":
		return "bool", true
	case "AUS":
		return "bool", false
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return "float", f
	}
	return "string", s
}
