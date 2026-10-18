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
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixlheureux/outpost/internal/codexadapter"
	"github.com/felixlheureux/outpost/internal/gitcli"
	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/httpapi"
	"github.com/felixlheureux/outpost/internal/privateaccess"
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
	var configPath string
	var workspacesDir string
	serve := &cobra.Command{Use: "serve", Short: "Serve the dashboard through a private Tailscale Serve boundary", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		host, _, err := net.SplitHostPort(listen)
		if err != nil {
			return fmt.Errorf("invalid --listen address: %w", err)
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("serve requires a numeric loopback --listen address")
		}
		var access *privateaccess.Config
		mode := "development"
		if !dev {
			config, err := privateaccess.Load(configPath)
			if err != nil {
				return err
			}
			if err := privateaccess.CheckNode(cmd.Context(), config); err != nil {
				return err
			}
			access = &config
			mode = "private"
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
		server := &http.Server{Addr: listen, Handler: httpapi.NewWithRepositorySetup(db, web.Assets(), Version, logger, access, stateDir, workspacesDir), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 11 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
		listener, err := net.Listen("tcp", listen)
		if err != nil {
			return err
		}
		finished := make(chan error, 1)
		go func() { finished <- server.Serve(listener) }()
		logger.Info("dashboard listening", "address", listener.Addr().String(), "mode", mode)
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
	serve.Flags().StringVar(&listen, "listen", "127.0.0.1:8080", "Loopback listen address")
	serve.Flags().StringVar(&configPath, "config", privateaccess.DefaultPath(), "Private access configuration file")
	serve.Flags().StringVar(&workspacesDir, "workspaces-dir", defaultUserPath("XDG_DATA_HOME", "outpost/workspaces"), "Managed checkout directory")
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
		check("User systemd bus", "start a real user session or enable the user manager", func() error {
			_, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "list-jobs", "--no-legend")
			return err
		})
		check("Boot persistence", "ask the host administrator to run loginctl enable-linger for this service user", func() error {
			owner, err := user.Current()
			if err != nil {
				return err
			}
			b, err := hostcmd.Run(ctx, "", "loginctl", nil, "show-user", owner.Username, "--property=Linger", "--value")
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(b)) != "yes" {
				return fmt.Errorf("lingering is disabled")
			}
			return nil
		})
		check("GitHub access", "check gh auth status and network access; sign in with gh auth login when needed", func() error { return githubcli.CheckAuth(ctx) })
		check("Codex provider", "install Codex 0.163.0 and sign in using codex login as the service user", func() error {
			path, err := codexadapter.FindExecutable()
			if err != nil {
				return err
			}
			if _, err := codexadapter.Version(ctx, path); err != nil {
				return err
			}
			return codexadapter.AuthStatus(ctx, path)
		})
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
		check("Private dashboard route", "configure the pinned Tailscale origin/login and a private Serve route (see INSTALL.md)", func() error {
			config, err := privateaccess.Load(configPath)
			if err != nil {
				return err
			}
			if err := privateaccess.CheckNode(ctx, config); err != nil {
				return err
			}
			present, err := privateaccess.ServeRoute(ctx, config, "http://127.0.0.1:8080")
			if err != nil {
				return err
			}
			if !present {
				return fmt.Errorf("Serve route is missing")
			}
			return nil
		})
		if repoURL != "" {
			check("Repository access", "check the URL and the active gh account", func() error { _, err := githubcli.GetRepository(ctx, repoURL); return err })
		}
		fmt.Fprintln(out, "Codex adapter readiness is checked. Start controls, validation, and delivery are separate integration stages.")
		if failures > 0 {
			return fmt.Errorf("%d setup check(s) need attention", failures)
		}
		return nil
	}}
	doctor.Flags().StringVar(&repoURL, "repo", "", "Also check read access to a GitHub repository URL")
	doctor.Flags().StringVar(&configPath, "config", privateaccess.DefaultPath(), "Private access configuration file")
	root.AddCommand(doctor)
	addInstall(root, &stateDir, out)
	addStatus(root, &stateDir, out)
	addStop(root, &stateDir, out)
	addWorker(root, &stateDir)
	addRun(root, &stateDir, out)
	return root
}
