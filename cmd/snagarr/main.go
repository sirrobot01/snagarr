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
	"github.com/sirrobot01/snagarr/internal/httpx"
	"github.com/sirrobot01/snagarr/internal/reconcile"
	"github.com/sirrobot01/snagarr/internal/resolver"
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

	httpx.UserAgent = version.UserAgent()
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
	if err := bootstrapAdmin(ctx, db, settings, cfg, log); err != nil {
		return err
	}

	engine := reconcile.New(db, settings, log)
	server := api.New(db, settings, resolver.New(db, log), engine, web.Handler(), log)

	go engine.Start(ctx)

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

// bootstrapAdmin makes the first run self-serve: one admin, one token, and a
// URL that carries the token so the operator never copies it by hand.
func bootstrapAdmin(ctx context.Context, db *store.Store, settings *config.Manager, cfg config.Config, log *slog.Logger) error {
	users, err := db.Users(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil
	}

	admin := &store.User{DisplayName: "Admin", Role: store.RoleAdmin}
	if err := db.CreateUser(ctx, admin); err != nil {
		return err
	}
	_, secret, err := db.CreateToken(ctx, admin.ID, "Setup token")
	if err != nil {
		return err
	}

	base := settings.Get().General.PublicURL
	if base == "" {
		base = "http://localhost" + cfg.Addr
	}
	log.Info("created the admin user")
	fmt.Printf("\n  Snagarr is ready. Open the setup URL to finish:\n\n    %s/#token=%s\n\n  Admin token: %s\n  Keep it safe — it is not shown again.\n\n",
		base, secret, secret)
	return nil
}
