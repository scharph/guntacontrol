// Command guntacontrol polls a Guntamatic Biostar Flex over its local HTTP API and
// re-exposes the live data as a JSON API, a Prometheus /metrics endpoint, an embedded
// web UI and a Loxone-friendly plain-text interface. It can also write boiler
// parameters when the configuration enables writing.
//
// All settings live in a YAML configuration file (see guntacontrol.example.yaml);
// there are no per-setting command-line flags.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"guntacontrol/internal/api"
	"guntacontrol/internal/config"
	"guntacontrol/internal/device"
	"guntacontrol/internal/loxone"
	"guntacontrol/internal/service"
	"guntacontrol/internal/store"
	"guntacontrol/web"
)

var version = "dev"

func main() {
	cfgPath := flag.String("config", "guntacontrol.yaml", "YAML configuration file (created on first save)")
	dataDir := flag.String("data", "", "data directory (overrides dataDir in the configuration)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	if err := run(*cfgPath, *dataDir); err != nil {
		log.Fatal(err)
	}
}

func run(cfgPath, dataDirOverride string) error {
	cfgStore, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config %s: %w", cfgPath, err)
	}
	if dataDirOverride != "" {
		if _, err := cfgStore.Update(func(c *config.Config) { c.DataDir = dataDirOverride }); err != nil {
			return err
		}
	}
	cfg := cfgStore.Get()

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory %q (must be writable by this user): %w", cfg.DataDir, err)
	}

	fb := device.NewFlexBackend(cfg.Device.Host, cfg.Device.Key, cfg.Device.WriteKey, cfg.Timeout())
	poller := service.NewPoller(fb, cfg.PollInterval())

	// Shared SQLite database backs both the Auswertung history and the audit log.
	db, err := store.OpenDB(cfg.DBPath())
	if err != nil {
		return fmt.Errorf("open database %s: %w", cfg.DBPath(), err)
	}
	defer db.Close()

	hist := store.NewHistory(db, cfg.PollInterval(), cfg.Retention(), store.PelletModel{
		NominalKW:  cfg.Pellets.NominalKW,
		Efficiency: cfg.Pellets.Efficiency,
		KWhPerKg:   cfg.Pellets.KWhPerKg,
	})
	poller.AddSink(hist)

	// Optional UDP push to a Loxone Miniserver on every poll.
	if cfg.Loxone.UDP != "" {
		if u, err := loxone.NewUDP(cfg.Loxone.UDP); err != nil {
			log.Printf("loxone-udp: disabled (%v)", err)
		} else {
			poller.AddSink(u)
			log.Printf("loxone udp push -> %s", cfg.Loxone.UDP)
		}
	}

	audit := store.NewAuditLog(db)

	srv := api.NewServer(poller, fb, fb, hist, audit, web.FS(), cfgStore)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go poller.Run(ctx)
	go hist.RunMaintenance(ctx.Done(), 10*time.Minute)

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	log.Printf("guntacontrol %s on %s -> boiler %s (poll %s, allowWrite=%v, readOnly=%v)",
		version, cfg.Listen, cfg.Device.Host, cfg.PollInterval(), cfg.AllowWrite, cfg.IsReadOnly())
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	log.Print("shut down")
	return nil
}
