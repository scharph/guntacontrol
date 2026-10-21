// Command guntacontrol polls a Guntamatic Biostar Flex over its local HTTP API and
// re-exposes the live data as a JSON API, a Prometheus /metrics endpoint, an embedded
// web UI and a Loxone-friendly plain-text interface. It can also write boiler
// parameters when started with -allow-write.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"guntacontrol/internal/api"
	"guntacontrol/internal/device"
	"guntacontrol/internal/loxone"
	"guntacontrol/internal/service"
	"guntacontrol/internal/store"
	"guntacontrol/web"
)

var version = "dev"

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
		dbFile   = flag.String("db", "guntacontrol.db", "SQLite database for the Auswertung history and the audit log")
		retain   = flag.Duration("retention", 35*24*time.Hour, "how long to keep history samples")
		loxUDP   = flag.String("loxone-udp", "", "push values to a Loxone Miniserver as UDP (host:port, e.g. 192.168.0.10:7000; empty = off)")
	)
	flag.Parse()

	fb := device.NewFlexBackend(*host, *key, *writeKey, *timeout)
	poller := service.NewPoller(fb, *interval)

	// Shared SQLite database backs both the Auswertung history and the audit log.
	db, err := store.OpenDB(*dbFile)
	if err != nil {
		log.Fatalf("open database %s: %v", *dbFile, err)
	}
	defer db.Close()

	hist := store.NewHistory(db, *interval, *retain)
	poller.AddSink(hist)

	// Optional UDP push to a Loxone Miniserver on every poll.
	if *loxUDP != "" {
		if u, err := loxone.NewUDP(*loxUDP); err != nil {
			log.Printf("loxone-udp: disabled (%v)", err)
		} else {
			poller.AddSink(u)
			log.Printf("loxone udp push -> %s", *loxUDP)
		}
	}

	audit := store.NewAuditLog(db)

	srv := api.NewServer(poller, fb, fb, hist, audit, web.FS(), *allowW)
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

	log.Printf("guntacontrol %s on %s -> boiler %s (poll %s, write=%v)",
		version, *addr, *host, *interval, *allowW)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Print("shut down")
}
