// Package cli is the composition root. Business logic must not depend on Cobra.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/felixlheureux/outpost/internal/gitcli"
	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/httpapi"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/web"
	"github.com/spf13/cobra"
)

var Version = "development"

func defaultStateDir() string {
	if root := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, "outpost")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "outpost")
}

func New(out, errOut io.Writer) *cobra.Command {
	stateDir := defaultStateDir()
	root := &cobra.Command{Use: "outpost", Short: "A private control room for coding agents", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&stateDir, "state-dir", stateDir, "Private local state directory")
	open := func(ctx context.Context) (*store.Store, error) {
		if !filepath.IsAbs(stateDir) {
			return nil, fmt.Errorf("--state-dir must be an absolute path")
		}
		return store.Open(ctx, stateDir)
	}
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print the executable version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { _, err := fmt.Fprintln(out, Version); return err }})
	root.AddCommand(&cobra.Command{Use: "migrate", Short: "Apply schema upgrades with web/worker stopped", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := open(cmd.Context())
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.Migrate(cmd.Context()); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, "Local schema is current.")
		return err
	}})
	var dev bool
	var listen string
	serve := &cobra.Command{Use: "serve", Short: "Serve the dashboard (starter: explicit local development only)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if !dev {
			return fmt.Errorf("private production access is not implemented yet; use serve --dev on loopback for this starter")
		}
		host, _, err := net.SplitHostPort(listen)
		if err != nil {
			return fmt.Errorf("invalid --listen address: %w", err)
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("development mode requires a numeric loopback --listen address")
		}
		db, err := open(cmd.Context())
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.CheckSchema(cmd.Context()); err != nil {
			return err
		}
		logger := slog.New(slog.NewJSONHandler(errOut, &slog.HandlerOptions{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if len(groups) == 0 && a.Key == slog.TimeKey {
					a.Value = slog.TimeValue(a.Value.Time().UTC())
				}
				return a
			},
		}))
		server := &http.Server{Addr: listen, Handler: httpapi.New(db, web.Assets(), Version, logger), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
		listener, err := net.Listen("tcp", listen)
		if err != nil {
			return err
		}
		finished := make(chan error, 1)
		go func() { finished <- server.Serve(listener) }()
		logger.Info("dashboard listening", "address", listener.Addr().String(), "mode", "development")
		select {
		case err := <-finished:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-cmd.Context().Done():
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(ctx)
		}
	}}
	serve.Flags().BoolVar(&dev, "dev", false, "Enable unauthenticated loopback development mode")
	serve.Flags().StringVar(&listen, "listen", "127.0.0.1:7331", "Loopback listen address")
	root.AddCommand(serve)
	var repoURL string
	doctor := &cobra.Command{Use: "doctor", Short: "Check tools and existing logins without changing them", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		failures := 0
		check := func(label, hint string, fn func() error) {
			if err := fn(); err != nil {
				failures++
				fmt.Fprintf(out, "[needs attention] %s — %s\n", label, hint)
			} else {
				fmt.Fprintf(out, "[ready] %s\n", label)
			}
		}
		ctx := cmd.Context()
		check("Git", "install git", func() error { _, err := gitcli.Version(ctx); return err })
		check("GitHub login", "run gh auth login --hostname github.com --web --git-protocol https, then gh auth setup-git", func() error { return githubcli.CheckAuth(ctx) })
		check("Codex login", "install Codex and sign in using its CLI", func() error { _, err := hostcmd.Run(ctx, "", "codex", nil, "login", "status"); return err })
		check("Tailscale login", "install Tailscale and run tailscale up", func() error {
			b, err := hostcmd.Run(ctx, "", "tailscale", nil, "status", "--json")
			if err != nil {
				return err
			}
			var status struct {
				BackendState string
				TailscaleIPs []string
			}
			if err := json.Unmarshal(b, &status); err != nil {
				return err
			}
			if status.BackendState != "Running" || len(status.TailscaleIPs) == 0 {
				return fmt.Errorf("Tailscale is not signed in and running")
			}
			return nil
		})
		if repoURL != "" {
			check("Repository access", "check the URL and the active gh account", func() error { _, err := githubcli.GetRepository(ctx, repoURL); return err })
		}
		fmt.Fprintln(out, "Starter status: local UI only. Installer, Tailscale identity and worker are not implemented.")
		if failures > 0 {
			return fmt.Errorf("%d setup check(s) need attention", failures)
		}
		return nil
	}}
	doctor.Flags().StringVar(&repoURL, "repo", "", "Also check read access to a GitHub repository URL")
	root.AddCommand(doctor)
	return root
}
