// Command snagarr runs the Snagarr server: the API, the reconcile loop and the
// embedded web client, from one binary.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirrobot01/snagarr/internal/api"
	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/engine"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
	"github.com/sirrobot01/snagarr/internal/version"
	"github.com/sirrobot01/snagarr/internal/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "snagarr:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}

	switch command {
	case "serve":
		return serve(args)
	case "version":
		fmt.Println("snagarr", version.String())
		return nil
	default:
		return fmt.Errorf("unknown command %q; try `snagarr serve` or `snagarr version`", command)
	}
}

func serve(args []string) error {
	cfg := config.Load()

	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.StringVar(&cfg.Addr, "addr", cfg.Addr, "address to listen on")
	flags.StringVar(&cfg.DataDir, "data", cfg.DataDir, "directory for the database and secret key")
	if err := flags.Parse(args); err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	integration.UserAgent = version.UserAgent()
	api.SetVersion(version.Version)

	key, err := cfg.SecretKey()
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.DatabasePath(), key)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	settings, err := config.NewManager(ctx, db)
	if err != nil {
		return err
	}
	// Existing installations may already have an admin to own environment
	// integrations. On a fresh install these are seeded after web registration.
	if err := settings.SeedServices(ctx, db); err != nil {
		return err
	}

	reconciler := engine.NewReconciler(db, settings, log)
	server := api.New(db, settings, engine.NewResolver(db, log), reconciler, web.Handler(), log)

	go reconciler.Start(ctx)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("snagarr listening", "addr", cfg.Addr, "version", version.Version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
