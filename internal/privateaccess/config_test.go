package privateaccess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	good := Config{Origin: "https://host.tailnet.ts.net", AllowedLogin: "owner@example.com"}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Config{
		{Origin: "http://host.tailnet.ts.net", AllowedLogin: good.AllowedLogin},
		{Origin: "https://host.tailnet.ts.net/path", AllowedLogin: good.AllowedLogin},
		{Origin: "https://host.tailnet.ts.net.evil.example", AllowedLogin: good.AllowedLogin},
		{Origin: "https://.ts.net", AllowedLogin: good.AllowedLogin},
		{Origin: good.Origin, AllowedLogin: "owner@example.com,other@example.com"},
		{Origin: good.Origin, AllowedLogin: ""},
	} {
		if err := c.Validate(); err == nil {
			t.Fatalf("accepted invalid config: %+v", c)
		}
	}
}

func TestSaveOnceKeepsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "config.json")
	c := Config{Origin: "https://host.tailnet.ts.net", AllowedLogin: "owner@example.com"}
	if err := SaveOnce(path, c); err != nil {
		t.Fatal(err)
	}
	if err := SaveOnce(path, c); err != nil {
		t.Fatal("repeat setup:", err)
	}
	if err := SaveOnce(path, Config{Origin: c.Origin, AllowedLogin: "other@example.com"}); err == nil {
		t.Fatal("changed the allowed operator")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("config mode: %v %v", info, err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("accepted public config file")
	}
}

func TestInspectServeRoute(t *testing.T) {
	c := Config{Origin: "https://host.tailnet.ts.net", AllowedLogin: "owner@example.com"}
	for _, tc := range []struct {
		name, data      string
		present, failed bool
	}{
		{"empty", `{}`, false, false},
		{"owned", `{"TCP":{"443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:7331"}}}}}`, true, false},
		{"wrong upstream", `{"TCP":{"443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:9000"}}}}}`, false, true},
		{"unrelated route preserved", `{"TCP":{"8443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:8443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:9000"}}}}}`, false, false},
		{"occupied port", `{"TCP":{"443":{"HTTPS":true}}}`, false, true},
		{"public funnel", `{"AllowFunnel":{"host.tailnet.ts.net:443":true}}`, false, true},
		{"foreground route", `{"Foreground":{"session":{"TCP":{"443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:7331"}}}}}}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			present, err := InspectServeRoute([]byte(tc.data), c, "http://127.0.0.1:7331")
			if present != tc.present || (err != nil) != tc.failed {
				t.Fatalf("present=%v err=%v", present, err)
			}
		})
	}
}

func TestInspectReplacementRequiresExactRoot(t *testing.T) {
	c := Config{Origin: "https://host.tailnet.ts.net", AllowedLogin: "owner@example.com"}
	previous := "http://127.0.0.1:8080"
	for _, tc := range []struct {
		name, data string
		allowed    bool
	}{
		{"authorized previous route", `{"TCP":{"443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:8080"}}}}}`, true},
		{"changed upstream", `{"TCP":{"443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:9000"}}}}}`, false},
		{"additional mount", `{"TCP":{"443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:8080"},"/other":{"Proxy":"http://127.0.0.1:9000"}}}}}`, false},
		{"funnel", `{"TCP":{"443":{"HTTPS":true}},"Web":{"host.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:8080"}}}},"AllowFunnel":{"host.tailnet.ts.net:443":true}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := InspectReplacement([]byte(tc.data), c, previous)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v, error=%v", tc.allowed, err)
			}
		})
	}
}
