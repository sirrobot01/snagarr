// Package cli defines Snagarr's Cobra command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirrobot01/snagarr/internal/api"
	"github.com/sirrobot01/snagarr/internal/bot"
	"github.com/sirrobot01/snagarr/internal/config"
	"github.com/sirrobot01/snagarr/internal/engine"
	"github.com/sirrobot01/snagarr/internal/integration"
	"github.com/sirrobot01/snagarr/internal/store"
	"github.com/sirrobot01/snagarr/internal/version"
	"github.com/sirrobot01/snagarr/internal/web"
	"github.com/spf13/cobra"
)

type serveOptions struct {
	addr    string
	dataDir string
}

func New() *cobra.Command {
	defaults := config.Load()
	rootOptions := serveOptions{addr: defaults.Addr, dataDir: defaults.DataDir}

	root := &cobra.Command{
		Use:           "snagarr",
		Short:         "Capture now. Watch later.",
		Long:          "Snagarr captures what you want to watch and connects it to your media stack.\nRun without a command to start the server.",
		Version:       version.String(),
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		// No arguments has always started the server, and release containers rely
		// on that contract. Interactive users still get help with --help.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServer(cmd, rootOptions)
		},
	}
	root.SetVersionTemplate("snagarr {{.Version}}\n")
	root.CompletionOptions.DisableDescriptions = true
	addServeFlags(root, &rootOptions)
	root.AddCommand(
		newServeCommand(defaults),
		newVersionCommand(),
		newLoginCommand(),
		newLogoutCommand(),
		newSnagCommand(),
		newListCommand(),
		newStatusCommand(),
	)
	return root
}

func newServeCommand(defaults config.Config) *cobra.Command {
	options := serveOptions{addr: defaults.Addr, dataDir: defaults.DataDir}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Snagarr server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServer(cmd, options)
		},
	}
	addServeFlags(cmd, &options)
	return cmd
}

func addServeFlags(cmd *cobra.Command, options *serveOptions) {
	cmd.Flags().StringVar(&options.addr, "addr", options.addr, "address to listen on")
	cmd.Flags().StringVar(&options.dataDir, "data", options.dataDir, "directory for the database and secret key")
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "snagarr", version.String())
		},
	}
}

func runServer(cmd *cobra.Command, options serveOptions) error {
	cfg := config.Load()
	cfg.Addr = options.addr
	cfg.DataDir = options.dataDir

	log := slog.New(slog.NewTextHandler(cmd.OutOrStdout(), &slog.HandlerOptions{Level: cfg.LogLevel}))
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

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	settings, err := config.NewManager(ctx, db)
	if err != nil {
		return err
	}
	if err := settings.SeedServices(ctx, db); err != nil {
		return err
	}

	reconciler := engine.NewReconciler(db, settings, log)
	resolver := engine.NewResolver(db, log)
	server := api.New(db, settings, resolver, reconciler, web.Handler(), log)
	go reconciler.Start(ctx)
	// The bot naps until a token is configured, so it always starts.
	go bot.New(db, settings, resolver, reconciler,
		engine.NewSender(db, settings, reconciler, log), log).Run(ctx)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("snagarr listening", "addr", cfg.Addr, "version", version.Version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

func PrintError(out io.Writer, err error) {
	p := newPrinter(out, false)
	fmt.Fprintf(out, "%s %v\n", p.paint(ansiRed, "Error:"), err)
}
