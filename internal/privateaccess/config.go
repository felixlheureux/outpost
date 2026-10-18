// Package privateaccess validates the fixed Tailscale Serve boundary for Outpost.
package privateaccess

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Config contains only public routing identity. Credentials remain with Tailscale.
type Config struct {
	Origin       string `json:"origin"`
	AllowedLogin string `json:"allowed_login"`
}

func DefaultPath() string {
	if root := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, "outpost", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "outpost", "config.json")
}

func Load(path string) (Config, error) {
	var c Config
	if !filepath.IsAbs(path) {
		return c, fmt.Errorf("config path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return c, fmt.Errorf("stat private access config: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return c, fmt.Errorf("private access config must be a regular file with mode 600")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("read private access config: %w", err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("decode private access config: %w", err)
	}
	return c, c.Validate()
}

// SaveOnce creates private configuration atomically. Repeated setup accepts the
// same identity and refuses to silently retarget an existing installation.
func SaveOnce(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("config path must be absolute")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("config directory must be private (chmod 700 %s)", dir)
	}
	if existing, err := Load(path); err == nil {
		if existing == c {
			return nil
		}
		return fmt.Errorf("existing private access config differs; inspect it before changing identity")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Link(f.Name(), path); err != nil {
		if existing, loadErr := Load(path); loadErr == nil && existing == c {
			return nil
		}
		return fmt.Errorf("publish private access config: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	u, err := url.Parse(c.Origin)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" || !strings.HasSuffix(u.Hostname(), ".ts.net") {
		return fmt.Errorf("origin must be an exact https://<node>.<tailnet>.ts.net origin")
	}
	if net.ParseIP(u.Hostname()) != nil || strings.ToLower(c.Origin) != c.Origin {
		return fmt.Errorf("origin must use the lowercase Tailscale DNS name")
	}
	labels := strings.Split(strings.TrimSuffix(u.Hostname(), ".ts.net"), ".")
	if len(labels) < 2 {
		return fmt.Errorf("origin must include a node and tailnet DNS label")
	}
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("origin contains an empty DNS label")
		}
	}
	if c.AllowedLogin == "" || strings.TrimSpace(c.AllowedLogin) != c.AllowedLogin || strings.ContainsAny(c.AllowedLogin, "\r\n, ") {
		return fmt.Errorf("allowed_login must be one exact Tailscale login")
	}
	return nil
}

func (c Config) Hostname() string {
	u, _ := url.Parse(c.Origin)
	return u.Hostname()
}
