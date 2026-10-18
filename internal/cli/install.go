package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixlheureux/outpost/internal/gitcli"
	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/privateaccess"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/spf13/cobra"
)

type installOptions struct {
	configPath, stateDir, workspacesDir, binaryPath string
	allowedLogin, replaceServeFrom                  string
	prepare, installDeps                            bool
}

func defaultUserPath(base, suffix string) string {
	if root := os.Getenv(base); filepath.IsAbs(root) {
		return filepath.Join(root, suffix)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch base {
	case "XDG_DATA_HOME":
		return filepath.Join(home, ".local", "share", suffix)
	default:
		return filepath.Join(home, ".local", "bin", suffix)
	}
}

func addInstall(root *cobra.Command, stateDir *string, out io.Writer) {
	opts := installOptions{}
	cmd := &cobra.Command{Use: "install", Short: "Prepare the private host service and verify its access route", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		opts.stateDir = *stateDir
		return runInstall(cmd.Context(), opts, out)
	}}
	cmd.Flags().StringVar(&opts.configPath, "config", privateaccess.DefaultPath(), "Private access configuration file")
	cmd.Flags().StringVar(&opts.workspacesDir, "workspaces-dir", defaultUserPath("XDG_DATA_HOME", "outpost/workspaces"), "Managed workspace directory")
	cmd.Flags().StringVar(&opts.binaryPath, "binary", defaultUserPath("XDG_BIN_HOME", "outpost"), "Installed executable path")
	cmd.Flags().StringVar(&opts.allowedLogin, "allowed-login", "", "Exact allowed Tailscale user login (defaults to node owner)")
	cmd.Flags().StringVar(&opts.replaceServeFrom, "replace-serve-from", "", "Expected old root proxy to replace, such as http://127.0.0.1:9000")
	cmd.Flags().BoolVar(&opts.prepare, "prepare", false, "Install and verify the local service without changing Tailscale Serve")
	cmd.Flags().BoolVar(&opts.installDeps, "install-deps", true, "Install missing supported host tools through scoped package or official CLI installers")
	root.AddCommand(cmd)
}

func runInstall(ctx context.Context, opts installOptions, out io.Writer) error {
	if err := checkInstallHost(ctx, opts.installDeps, out); err != nil {
		return err
	}
	origin, owner, err := privateaccess.NodeIdentity(ctx)
	if err != nil {
		return err
	}
	login := opts.allowedLogin
	if login == "" {
		login = owner
	}
	access := privateaccess.Config{Origin: origin, AllowedLogin: login}
	if err := access.Validate(); err != nil {
		return fmt.Errorf("choose an allowed Tailscale login with --allowed-login: %w", err)
	}
	for _, path := range []string{opts.configPath, opts.stateDir, opts.workspacesDir, opts.binaryPath} {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "\r\n\t %") {
			return fmt.Errorf("installation paths must be absolute and contain no whitespace or systemd percent escapes: %s", path)
		}
	}
	if err := privateaccess.SaveOnce(opts.configPath, access); err != nil {
		return err
	}
	if err := ensurePrivateDir(opts.workspacesDir); err != nil {
		return err
	}
	if err := installBinary(opts.binaryPath); err != nil {
		return err
	}
	db, err := store.Open(ctx, opts.stateDir)
	if err != nil {
		return err
	}
	active, err := outpostWebActive(ctx)
	if err != nil {
		db.Close()
		return err
	}
	if active {
		err = db.CheckSchema(ctx)
	} else {
		err = db.Migrate(ctx)
	}
	if err != nil {
		db.Close()
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	if err := installWebUnit(ctx, opts); err != nil {
		return err
	}
	if err := verifyLocalWeb(ctx, access); err != nil {
		return err
	}
	fmt.Fprintf(out, "Private web service prepared on 127.0.0.1:8080 for %s.\n", login)
	if opts.prepare {
		fmt.Fprintln(out, "Tailscale Serve was not changed. Inspect its current route before cutover.")
		return nil
	}
	if err := configureServe(ctx, access, opts); err != nil {
		return err
	}
	if err := verifyPrivateURL(ctx, access); err != nil {
		return fmt.Errorf("Serve points to Outpost, but allowed-user HTTPS verification is incomplete: %w", err)
	}
	fmt.Fprintf(out, "Verified private dashboard: %s (allowed login: %s)\n", access.Origin, login)
	return nil
}

func checkInstallHost(ctx context.Context, installDeps bool, out io.Writer) error {
	if err := ensurePrerequisites(ctx, installDeps, out); err != nil {
		return err
	}
	if _, err := gitcli.Version(ctx); err != nil {
		return err
	}
	if err := githubcli.CheckAuth(ctx); err != nil {
		return fmt.Errorf("GitHub CLI sign-in is unavailable; run gh auth login as this user, then rerun install: %w", err)
	}
	if _, err := hostcmd.Run(ctx, "", "codex", nil, "login", "status"); err != nil {
		return fmt.Errorf("Codex sign-in is unavailable; run codex login as this user, then rerun install: %w", err)
	}
	if _, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "list-jobs", "--no-legend"); err != nil {
		return fmt.Errorf("user systemd bus is unavailable: %w", err)
	}
	serviceUser, err := user.Current()
	if err != nil {
		return err
	}
	linger, err := hostcmd.Run(ctx, "", "loginctl", nil, "show-user", serviceUser.Username, "--property=Linger", "--value")
	if err != nil || strings.TrimSpace(string(linger)) != "yes" {
		fmt.Fprintf(out, "Boot persistence missing; action: sudo loginctl enable-linger %s\n", serviceUser.Username)
		if !installDeps {
			return fmt.Errorf("boot persistence is unavailable; run the displayed action and rerun install")
		}
		if _, err := exec.LookPath("sudo"); err != nil {
			return fmt.Errorf("sudo is unavailable; ask the host administrator to run loginctl enable-linger %s", serviceUser.Username)
		}
		if err := runInstallStep(ctx, out, "sudo", "loginctl", "enable-linger", serviceUser.Username); err != nil {
			return err
		}
		linger, err = hostcmd.Run(ctx, "", "loginctl", nil, "show-user", serviceUser.Username, "--property=Linger", "--value")
		if err != nil || strings.TrimSpace(string(linger)) != "yes" {
			return fmt.Errorf("boot persistence is still unavailable after loginctl enable-linger")
		}
	}
	return nil
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("directory must be private (chmod 700 %s)", path)
	}
	return nil
}

func installBinary(target string) error {
	source, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("installed binary target is not a regular file: %s", target)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	sourceBytes, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if installed, err := os.ReadFile(target); err == nil {
		if sha256.Sum256(installed) == sha256.Sum256(sourceBytes) {
			return nil
		}
		return fmt.Errorf("installed binary differs; stop and reconcile active work before upgrading %s", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".outpost-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0700); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(sourceBytes); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Link(tmp.Name(), target)
}

func outpostWebActive(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", "--quiet", "outpost.service")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && (exit.ExitCode() == 3 || exit.ExitCode() == 4) {
		return false, nil
	}
	return false, fmt.Errorf("cannot establish Outpost web service state: %w", err)
}

func installWebUnit(ctx context.Context, opts installOptions) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0700); err != nil {
		return err
	}
	legacyUnit := "[Unit]\nDescription=Outpost private dashboard\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nExecStart=" + opts.binaryPath + " --state-dir " + opts.stateDir + " serve --config " + opts.configPath + "\nRestart=on-failure\nRestartSec=3\nNoNewPrivileges=true\n\n[Install]\nWantedBy=default.target\n"
	unit := strings.Replace(legacyUnit, " serve --config "+opts.configPath+"\n", " serve --config "+opts.configPath+" --workspaces-dir "+opts.workspacesDir+"\n", 1)
	unitPath := filepath.Join(unitDir, "outpost.service")
	changed := false
	if existing, err := os.ReadFile(unitPath); err == nil {
		if string(existing) != unit && string(existing) != legacyUnit {
			return fmt.Errorf("existing outpost.service differs; reconcile it before installation")
		}
		changed = string(existing) != unit
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0600); err != nil {
		return err
	}
	if _, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "daemon-reload"); err != nil {
		return err
	}
	if _, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "enable", "--now", "outpost.service"); err != nil {
		return err
	}
	if changed {
		if _, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "restart", "outpost.service"); err != nil {
			return err
		}
	}
	return nil
}

func verifyLocalWeb(ctx context.Context, c privateaccess.Config) error {
	client := &http.Client{Timeout: 5 * time.Second}
	for attempt := 0; attempt < 10; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8080/api/v1/bootstrap", nil)
		if err != nil {
			return err
		}
		req.Host = c.Hostname()
		req.Header.Set("Tailscale-User-Login", c.AllowedLogin)
		res, err := client.Do(req)
		if err == nil {
			var body struct {
				Mode string `json:"mode"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(res.Body, 16<<10)).Decode(&body)
			res.Body.Close()
			if res.StatusCode == 200 && decodeErr == nil && body.Mode == "private" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return fmt.Errorf("local private dashboard API did not become ready")
}

func verifyPrivateURL(ctx context.Context, c privateaccess.Config) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Origin+"/api/v1/bootstrap", nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("private URL returned HTTP %d", res.StatusCode)
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<10)).Decode(&body); err != nil {
		return err
	}
	if body.Mode != "private" {
		return fmt.Errorf("private URL did not serve Outpost")
	}
	page, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Origin+"/", nil)
	if err != nil {
		return err
	}
	page.Header.Set("Accept", "text/html")
	ui, err := client.Do(page)
	if err != nil {
		return err
	}
	defer ui.Body.Close()
	if ui.StatusCode != http.StatusOK || !strings.Contains(ui.Header.Get("Content-Type"), "text/html") {
		return fmt.Errorf("private URL did not serve the dashboard UI (HTTP %d)", ui.StatusCode)
	}
	return nil
}
