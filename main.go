// Command guntamatic is a small HTTP server that polls a Guntamatic Biostar boiler's
// WEB-Schnittstelle (HTTP/JSON interface on TCP 80) and re-exposes the live data as a
// JSON API, a Prometheus /metrics endpoint, and a plain status page. It can also write
// boiler parameters (controller program, boiler release, heating circuits, ...) when
// started with -allow-write.
//
// The boiler requires an access key ("Schlüssel") that Guntamatic derives from the
// boiler's Systemcode; its length sets the permission level (W0..W3). Pass it via
// -key or the GUNTAMATIC_KEY environment variable. Never hard-code it.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"guntamatic/web"
)

func main() {
	var (
		host     = flag.String("host", "192.168.0.69", "boiler IP or hostname")
		key      = flag.String("key", os.Getenv("GUNTAMATIC_KEY"), "read key (optional on the Flex; reads are open on the local network)")
		writeKey = flag.String("write-key", os.Getenv("GUNTAMATIC_WRITE_KEY"), "write key for /api/param control (or set GUNTAMATIC_WRITE_KEY)")
		addr     = flag.String("addr", ":8080", "listen address for this server")
		interval = flag.Duration("interval", 10*time.Second, "poll interval")
		timeout  = flag.Duration("timeout", 8*time.Second, "per-request timeout to the boiler")
		allowW   = flag.Bool("allow-write", false, "permit writes at all; also starts with write protection OFF")
		state    = flag.String("state", "guntamatic-state.json", "file persisting the write-protection toggle (empty = no persistence)")
		dbFile   = flag.String("db", "guntamatic.db", "SQLite database for the Auswertung history and the audit log")
		retain   = flag.Duration("retention", 35*24*time.Hour, "how long to keep history samples")
		loxUDP   = flag.String("loxone-udp", "", "push values to a Loxone Miniserver as UDP (host:port, e.g. 192.168.0.10:7000; empty = off)")
	)
	flag.Parse()

	// The Flex local CSV API serves reads without a key; the write key is only needed
	// for control (/api/param).
	fb := NewFlexBackend(*host, *key, *writeKey, *timeout)
	var (
		src   Source         = fb
		param paramWriter    = fb
		sched scheduleReader = fb
	)

	poller := NewPoller(src, *interval)

	// Shared SQLite database backs both the Auswertung history and the audit log.
	db, err := OpenDB(*dbFile)
	if err != nil {
		log.Fatalf("open database %s: %v", *dbFile, err)
	}
	defer db.Close()

	// Continuous history powers the Auswertung (evaluation) endpoints.
	hist := NewHistory(db, *interval, *retain)
	poller.AddSink(hist)

	// Optional UDP push to a Loxone Miniserver on every poll.
	if *loxUDP != "" {
		if u, err := NewLoxoneUDP(*loxUDP); err != nil {
			log.Printf("loxone-udp: disabled (%v)", err)
		} else {
			poller.AddSink(u)
			log.Printf("loxone udp push -> %s", *loxUDP)
		}
	}

	audit := NewAuditLog(db)

	srv := NewServer(poller, param, *allowW)
	srv.webFS = web.FS()
	srv.history = hist
	srv.sched = sched
	srv.audit = audit
	// Write protection defaults to ON (safe); -allow-write starts it OFF. A persisted
	// value from a previous run overrides this initial choice.
	srv.InitState(*state, !*allowW)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go poller.Run(ctx)
	go hist.RunMaintenance(ctx.Done(), 10*time.Minute)

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	log.Printf("guntacontrol on %s -> boiler %s (poll %s, write=%v)",
		*addr, *host, *interval, *allowW)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Print("shut down")
}
