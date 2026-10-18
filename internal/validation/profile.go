package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Check struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Argv               []string   `json:"argv"`
	Cwd                string     `json:"cwd"`
	Required           bool       `json:"required"`
	ExecutionClass     string     `json:"execution_class"`
	TimeoutSeconds     int        `json:"timeout_seconds"`
	RequireCases       bool       `json:"require_cases"`
	CaseCountPattern   string     `json:"case_count_pattern"`
	EnvKeys            []string   `json:"env_keys"`
	SupplementalInputs []string   `json:"supplemental_inputs"`
	ToolchainProbes    [][]string `json:"toolchain_probes"`
}
type Profile struct {
	ID          string  `json:"id"`
	Prepare     []Check `json:"prepare"`
	Checks      []Check `json:"checks"`
	RepairLimit int     `json:"repair_limit"`
}

var namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var envPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func Parse(raw []byte) (Profile, error) {
	var p Profile
	if len(raw) > 64<<10 {
		return p, fmt.Errorf("check profile is too large")
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("check profile needs one JSON object")
	}
	if p.RepairLimit == 0 {
		p.RepairLimit = 3
	}
	if !namePattern.MatchString(p.ID) || p.RepairLimit < 1 || p.RepairLimit > 10 || len(p.Checks) == 0 || len(p.Checks) > 20 || len(p.Prepare) > 10 {
		return p, fmt.Errorf("invalid check profile")
	}
	seen := map[string]bool{}
	for _, c := range append(append([]Check{}, p.Prepare...), p.Checks...) {
		if !namePattern.MatchString(c.ID) || seen[c.ID] || len(c.Argv) == 0 || len(c.Argv) > 32 || c.TimeoutSeconds < 0 || c.TimeoutSeconds > 3600 || len(c.EnvKeys) > 20 || len(c.SupplementalInputs) > 30 || len(c.ToolchainProbes) > 5 {
			return p, fmt.Errorf("invalid check definition %q", c.ID)
		}
		seen[c.ID] = true
		cwd := c.Cwd
		if cwd == "" {
			cwd = "."
		}
		if c.ExecutionClass != "local" || filepath.IsAbs(cwd) || !safeRelative(cwd) {
			return p, fmt.Errorf("check %s has unsupported class or unsafe cwd", c.ID)
		}
		for _, v := range c.Argv {
			if v == "" || strings.ContainsRune(v, 0) || len(v) > 4096 {
				return p, fmt.Errorf("check %s has invalid argv", c.ID)
			}
		}
		for _, v := range c.EnvKeys {
			upper := strings.ToUpper(v)
			if !envPattern.MatchString(v) || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "CREDENTIAL") || strings.Contains(upper, "API_KEY") {
				return p, fmt.Errorf("check %s has invalid env key", c.ID)
			}
		}
		for _, v := range c.SupplementalInputs {
			if !safeRelative(v) {
				return p, fmt.Errorf("check %s has unsafe input pattern", c.ID)
			}
		}
		for _, probe := range c.ToolchainProbes {
			if len(probe) == 0 || len(probe) > 8 {
				return p, fmt.Errorf("check %s has invalid toolchain probe", c.ID)
			}
			for _, arg := range probe {
				if arg == "" || len(arg) > 1000 || strings.ContainsRune(arg, 0) {
					return p, fmt.Errorf("check %s has invalid toolchain probe", c.ID)
				}
			}
		}
		if c.RequireCases {
			r, err := regexp.Compile(c.CaseCountPattern)
			if err != nil || r.NumSubexp() != 1 {
				return p, fmt.Errorf("check %s needs a case count pattern with one capture", c.ID)
			}
		}
	}
	for _, c := range p.Prepare {
		if c.RequireCases {
			return p, fmt.Errorf("prepare check %s cannot require cases", c.ID)
		}
	}
	required := 0
	for _, c := range p.Checks {
		if c.Required {
			required++
		}
	}
	if required == 0 {
		return p, fmt.Errorf("check profile needs at least one required check")
	}
	return p, nil
}
func Load(path string) (Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}
	return Parse(b)
}
func safeRelative(v string) bool {
	return v != "" && v != ".." && !strings.HasPrefix(v, "../") && !strings.Contains(v, "/../") && !strings.ContainsRune(v, 0)
}
func Digest(parts ...string) string {
	h := sha256.New()
	for _, v := range parts {
		fmt.Fprintf(h, "%d:%s", len(v), v)
	}
	return hex.EncodeToString(h.Sum(nil))
}
