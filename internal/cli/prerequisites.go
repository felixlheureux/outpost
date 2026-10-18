package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// prerequisitePlan is computed before making host changes. Existing tools are
// never replaced just to satisfy installation.
type prerequisitePlan struct {
	apt       []string
	tailscale bool
	codex     bool
}

func planPrerequisites(has func(string) bool) prerequisitePlan {
	var plan prerequisitePlan
	for _, pair := range [][2]string{{"git", "git"}, {"gh", "gh"}, {"curl", "curl"}} {
		if !has(pair[0]) {
			plan.apt = append(plan.apt, pair[1])
		}
	}
	plan.tailscale = !has("tailscale")
	plan.codex = !has("codex")
	return plan
}

func supportedInstallHost() error {
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return fmt.Errorf("guided installation supports Linux amd64/arm64")
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("run outpost install as the ordinary service user, not root")
	}
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return err
	}
	var distro string
	for _, line := range strings.Split(string(b), "\n") {
		if value, ok := strings.CutPrefix(line, "ID="); ok {
			distro = strings.Trim(value, `"'`)
		}
	}
	if distro != "ubuntu" && distro != "debian" {
		return fmt.Errorf("guided dependency installation supports Debian/Ubuntu; install Git, gh, Tailscale, Codex and user systemd manually")
	}
	for _, name := range []string{"systemctl", "loginctl", "apt-get"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("%s is unavailable; install systemd and apt support before running Outpost", name)
		}
	}
	return nil
}

func runInstallStep(ctx context.Context, out io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w; complete the displayed host action and rerun outpost install", name, err)
	}
	return nil
}

func installOfficialScript(ctx context.Context, out io.Writer, url string) error {
	f, err := os.CreateTemp("", "outpost-installer-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := runInstallStep(ctx, out, "curl", "--fail", "--show-error", "--silent", "--location", "--proto", "=https", "--tlsv1.2", url, "--output", f.Name()); err != nil {
		return err
	}
	return runInstallStep(ctx, out, "sh", f.Name())
}

func ensurePrerequisites(ctx context.Context, install bool, out io.Writer) error {
	if err := supportedInstallHost(); err != nil {
		return err
	}
	has := func(name string) bool { _, err := exec.LookPath(name); return err == nil }
	plan := planPrerequisites(has)
	if len(plan.apt) == 0 && !plan.tailscale && !plan.codex {
		fmt.Fprintln(out, "Host tools: existing Git, gh, Tailscale and Codex installations reused.")
		return nil
	}
	if len(plan.apt) > 0 {
		fmt.Fprintf(out, "Missing packages: %s; action: sudo apt-get update && sudo apt-get install -y %s ca-certificates\n", strings.Join(plan.apt, ", "), strings.Join(plan.apt, " "))
	}
	if plan.tailscale {
		fmt.Fprintln(out, "Missing Tailscale; action: official https://tailscale.com/install.sh installer as this user (it requests package privileges).")
	}
	if plan.codex {
		fmt.Fprintln(out, "Missing Codex; action: official https://chatgpt.com/codex/install.sh standalone installer as this user.")
	}
	if !install {
		return fmt.Errorf("host prerequisites are missing; rerun with --install-deps=true after reviewing the actions above")
	}
	if len(plan.apt) > 0 {
		if !has("sudo") {
			return fmt.Errorf("sudo is unavailable; ask the host administrator to install %s and rerun install", strings.Join(plan.apt, ", "))
		}
		if err := runInstallStep(ctx, out, "sudo", "apt-get", "update"); err != nil {
			return err
		}
		args := append([]string{"apt-get", "install", "-y", "ca-certificates"}, plan.apt...)
		if err := runInstallStep(ctx, out, "sudo", args...); err != nil {
			return err
		}
	}
	if plan.tailscale {
		if !has("sudo") {
			return fmt.Errorf("sudo is unavailable; install Tailscale with the host administrator and rerun install")
		}
		if err := installOfficialScript(ctx, out, "https://tailscale.com/install.sh"); err != nil {
			return err
		}
	}
	if plan.codex {
		if err := installOfficialScript(ctx, out, "https://chatgpt.com/codex/install.sh"); err != nil {
			return err
		}
	}
	for _, name := range []string{"git", "gh", "tailscale", "codex"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("%s is still unavailable after installation; check PATH and rerun as the same user", name)
		}
	}
	return nil
}
