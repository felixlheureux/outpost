package privateaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/felixlheureux/outpost/internal/hostcmd"
)

// CheckNode refuses startup when the configured private origin is not this node.
// Serve route ownership is checked separately during installation.
func CheckNode(ctx context.Context, c Config) error {
	origin, _, err := NodeIdentity(ctx)
	if err != nil {
		return err
	}
	if origin != c.Origin {
		return fmt.Errorf("configured origin does not match this Tailscale node")
	}
	return nil
}

// NodeIdentity observes this node's private HTTPS origin and its owning login.
// The owner is a setup default, not evidence about a visiting browser.
func NodeIdentity(ctx context.Context) (string, string, error) {
	b, err := hostcmd.Run(ctx, "", "tailscale", nil, "status", "--json")
	if err != nil {
		return "", "", fmt.Errorf("inspect Tailscale state: %w", err)
	}
	var status struct {
		BackendState string `json:"BackendState"`
		Self         struct {
			DNSName string `json:"DNSName"`
			UserID  int64  `json:"UserID"`
		} `json:"Self"`
		User map[string]struct {
			LoginName string `json:"LoginName"`
		} `json:"User"`
	}
	if err := json.Unmarshal(b, &status); err != nil {
		return "", "", fmt.Errorf("decode Tailscale state: %w", err)
	}
	if status.BackendState != "Running" {
		return "", "", fmt.Errorf("Tailscale is not running")
	}
	origin := "https://" + strings.TrimSuffix(strings.ToLower(status.Self.DNSName), ".")
	owner := status.User[fmt.Sprint(status.Self.UserID)].LoginName
	if err := (Config{Origin: origin, AllowedLogin: "operator@example.com"}).Validate(); err != nil {
		return "", "", fmt.Errorf("Tailscale node identity is incomplete: %w", err)
	}
	return origin, owner, nil
}

// ServeRoute checks only Outpost's root HTTPS mount. Other Serve routes are
// deliberately left intact.
func ServeRoute(ctx context.Context, c Config, upstream string) (bool, error) {
	b, err := hostcmd.Run(ctx, "", "tailscale", nil, "serve", "status", "--json")
	if err != nil {
		return false, fmt.Errorf("inspect Tailscale Serve: %w", err)
	}
	return InspectServeRoute(b, c, upstream)
}

func InspectServeRoute(b []byte, c Config, upstream string) (bool, error) {
	var status serveConfig
	if err := json.Unmarshal(b, &status); err != nil {
		return false, fmt.Errorf("decode Tailscale Serve state: %w", err)
	}
	hostPort := net.JoinHostPort(c.Hostname(), "443")
	for _, raw := range status.Foreground {
		var foreground serveConfig
		if err := json.Unmarshal(raw, &foreground); err != nil {
			return false, fmt.Errorf("decode Tailscale foreground Serve state: %w", err)
		}
		if foreground.AllowFunnel[hostPort] || foreground.Web[hostPort].Handlers != nil || foreground.TCP["443"] != nil {
			return false, fmt.Errorf("a temporary foreground Serve session owns Outpost's HTTPS endpoint")
		}
	}
	if status.AllowFunnel[hostPort] {
		return false, fmt.Errorf("Outpost's HTTPS endpoint has Funnel enabled; disable it before proceeding")
	}
	if web, exists := status.Web[hostPort]; exists {
		if root, exists := web.Handlers["/"]; exists && root.Proxy == upstream {
			return true, nil
		}
		return false, fmt.Errorf("Tailscale Serve already owns %s with a different root route", hostPort)
	}
	if _, exists := status.TCP["443"]; exists {
		return false, fmt.Errorf("Tailscale Serve already owns HTTPS port 443")
	}
	return false, nil
}

type serveConfig struct {
	TCP map[string]json.RawMessage `json:"TCP"`
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy"`
		} `json:"Handlers"`
	} `json:"Web"`
	AllowFunnel map[string]bool            `json:"AllowFunnel"`
	Foreground  map[string]json.RawMessage `json:"Foreground"`
}

// InspectReplacement permits a cutover only from the exact previously observed
// root proxy. It refuses other mounts, Funnel and temporary foreground routes.
func InspectReplacement(b []byte, c Config, previous string) error {
	var status serveConfig
	if err := json.Unmarshal(b, &status); err != nil {
		return fmt.Errorf("decode Tailscale Serve state: %w", err)
	}
	hostPort := net.JoinHostPort(c.Hostname(), "443")
	if len(status.Foreground) != 0 || status.AllowFunnel[hostPort] {
		return fmt.Errorf("temporary Serve or Funnel configuration prevents a safe cutover")
	}
	web, exists := status.Web[hostPort]
	if !exists || len(web.Handlers) != 1 || web.Handlers["/"].Proxy != previous {
		return fmt.Errorf("existing Serve route differs from the expected previous root proxy")
	}
	if _, exists := status.TCP["443"]; !exists {
		return fmt.Errorf("existing HTTPS port 443 configuration is missing")
	}
	return nil
}
