// Package loxone provides the plain-text formatting the Loxone Miniserver expects and
// a UDP pusher that forwards live values to a "Virtueller UDP Eingang".
package loxone

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"guntacontrol/internal/boiler"
)

// Key turns a channel name into a stable ASCII marker for Loxone start strings, folding
// German umlauts and dropping non-alphanumerics: "Puffer oben" -> "PufferOben".
func Key(name string) string {
	repl := strings.NewReplacer(
		"ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss",
	)
	var b strings.Builder
	for _, r := range repl.Replace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Value renders a channel value as plain text Loxone can parse: numbers as-is, booleans
// as 1/0, strings verbatim.
func Value(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "1"
		}
		return "0"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

// UDP pushes the current channel values to a Loxone Miniserver as UDP datagrams of
// plain "Key=Value" text. It implements service.SampleSink, so the poller feeds it on
// every successful poll.
type UDP struct {
	conn net.Conn
}

// udpMaxPayload keeps each datagram within a typical MTU; lines never straddle packets
// so every "Key=Value" marker stays intact for Loxone.
const udpMaxPayload = 1200

func NewUDP(addr string) (*UDP, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	return &UDP{conn: conn}, nil
}

func (u *UDP) Add(ts time.Time, vals []boiler.Value) {
	lines := make([]string, 0, len(vals)+2)
	lines = append(lines, "online=1")
	lines = append(lines, "updated="+strconv.FormatInt(ts.Unix(), 10))
	for _, v := range vals {
		key := Key(v.Name)
		if key == "" {
			continue
		}
		lines = append(lines, key+"="+Value(v.Value))
	}
	u.flush(lines)
}

func (u *UDP) flush(lines []string) {
	var b strings.Builder
	send := func() {
		if b.Len() == 0 {
			return
		}
		if _, err := u.conn.Write([]byte(b.String())); err != nil {
			log.Printf("loxone-udp: send failed: %v", err)
		}
		b.Reset()
	}
	for _, ln := range lines {
		if b.Len()+len(ln)+1 > udpMaxPayload {
			send()
		}
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	send()
}
