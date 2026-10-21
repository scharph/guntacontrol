package main

import (
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// loxoneUDP pushes the current channel values to a Loxone Miniserver as UDP datagrams
// of plain "Key=Value" text (a "Virtueller UDP Eingang" parses them by start marker,
// just like the HTTP input). It implements sampleSink, so the poller feeds it on every
// successful poll — no extra timer needed.
type loxoneUDP struct {
	conn net.Conn
}

// udpMaxPayload keeps each datagram well within a typical MTU; lines never straddle
// packets so every "Key=Value" marker stays intact for Loxone.
const udpMaxPayload = 1200

func NewLoxoneUDP(addr string) (*loxoneUDP, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	return &loxoneUDP{conn: conn}, nil
}

func (u *loxoneUDP) Add(ts time.Time, vals []Value) {
	lines := make([]string, 0, len(vals)+2)
	lines = append(lines, "online=1")
	lines = append(lines, "updated="+strconv.FormatInt(ts.Unix(), 10))
	for _, v := range vals {
		key := loxoneKey(v.Name)
		if key == "" {
			continue
		}
		lines = append(lines, key+"="+loxoneValue(v.Value))
	}
	u.flush(lines)
}

// flush sends the lines in as few datagrams as fit under udpMaxPayload.
func (u *loxoneUDP) flush(lines []string) {
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
