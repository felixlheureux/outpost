// Package issues parses GitHub Issue requirements without granting issue text
// authority to execute commands or alter operator configuration.
package issues

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const maxBodyBytes = 256 << 10

var policyName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type Settings struct {
	OutpostTask  int     `yaml:"outpost_task" json:"outpost_task"`
	Kind         string  `yaml:"kind" json:"kind"`
	DependsOn    []int   `yaml:"depends_on" json:"depends_on"`
	HoldReason   *string `yaml:"hold_reason" json:"hold_reason"`
	CheckProfile string  `yaml:"check_profile" json:"check_profile"`
	Delivery     string  `yaml:"delivery" json:"delivery"`
	Review       string  `yaml:"review" json:"review"`
}

type Requirements struct {
	Title    string   `json:"title"`
	Outcome  string   `json:"outcome"`
	Criteria []string `json:"criteria"`
	Notes    string   `json:"notes"`
	Settings Settings `json:"settings"`
	HasBlock bool     `json:"has_settings"`
}

func Parse(number int, title, body string) (Requirements, error) {
	if number <= 0 || len(title) > 500 || strings.TrimSpace(title) == "" || len(body) > maxBodyBytes {
		return Requirements{}, fmt.Errorf("issue title or body exceeds the supported bounds")
	}
	req := Requirements{Title: strings.TrimSpace(title), Outcome: strings.TrimSpace(body), Criteria: []string{}, Settings: Settings{Kind: "implementation", CheckProfile: "inherit", Delivery: "inherit", Review: "inherit"}}
	sections, counts := splitSections(body)
	if counts["outcome"] > 1 || counts["acceptance criteria"] > 1 || counts["implementation notes"] > 1 || counts["outpost settings"] > 1 {
		return Requirements{}, fmt.Errorf("issue has duplicate task sections")
	}
	if counts["outcome"] == 1 {
		req.Outcome = strings.TrimSpace(sections["outcome"])
	}
	if req.Outcome == "" {
		req.Outcome = req.Title
	}
	if counts["acceptance criteria"] == 1 {
		for _, line := range strings.Split(sections["acceptance criteria"], "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
				continue
			}
			item := strings.TrimSpace(line[2:])
			if len(item) >= 4 && item[0] == '[' && item[2] == ']' && item[3] == ' ' {
				item = strings.TrimSpace(item[4:])
			}
			if item == "" || len(item) > 4000 || len(req.Criteria) >= 50 {
				return Requirements{}, fmt.Errorf("acceptance criteria exceed supported bounds")
			}
			req.Criteria = append(req.Criteria, item)
		}
	}
	req.Notes = strings.TrimSpace(sections["implementation notes"])
	if counts["outpost settings"] == 0 {
		return req, nil
	}
	block, err := yamlBlock(sections["outpost settings"])
	if err != nil {
		return Requirements{}, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(block))
	decoder.KnownFields(true)
	if err := decoder.Decode(&req.Settings); err != nil {
		return Requirements{}, fmt.Errorf("invalid Outpost settings: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Requirements{}, fmt.Errorf("Outpost settings must contain one YAML document")
	}
	if err := validateSettings(number, req.Settings); err != nil {
		return Requirements{}, err
	}
	req.HasBlock = true
	return req, nil
}

func splitSections(body string) (map[string]string, map[string]int) {
	sections := make(map[string]string)
	counts := make(map[string]int)
	var current string
	var lines []string
	flush := func() {
		if current != "" {
			sections[current] = strings.Join(lines, "\n")
		}
		lines = nil
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			flush()
			current = strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "# ")))
			counts[current]++
			continue
		}
		if current != "" {
			lines = append(lines, line)
		}
	}
	flush()
	return sections, counts
}

func yamlBlock(section string) (string, error) {
	var block string
	inBlock := false
	closed := false
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```yaml" {
			if inBlock || closed {
				return "", fmt.Errorf("use one fenced YAML block under Outpost settings")
			}
			inBlock = true
			continue
		}
		if trimmed == "```" && inBlock {
			inBlock = false
			closed = true
			continue
		}
		if inBlock {
			block += line + "\n"
			if len(block) > 8192 {
				return "", fmt.Errorf("Outpost settings block is too large")
			}
			continue
		}
		if trimmed != "" {
			return "", fmt.Errorf("Outpost settings must contain only one fenced YAML block")
		}
	}
	if inBlock || strings.TrimSpace(block) == "" {
		return "", fmt.Errorf("Outpost settings need one complete fenced YAML block")
	}
	return block, nil
}

func validateSettings(number int, settings Settings) error {
	if settings.OutpostTask != 1 || settings.Kind != "implementation" {
		return fmt.Errorf("unsupported Outpost task version or kind")
	}
	if !policyName.MatchString(settings.CheckProfile) {
		return fmt.Errorf("invalid check profile reference")
	}
	switch settings.Delivery {
	case "inherit", "local_changes", "commit", "push":
	default:
		return fmt.Errorf("unsupported delivery policy")
	}
	switch settings.Review {
	case "inherit", "none", "required":
	default:
		return fmt.Errorf("unsupported review policy")
	}
	if settings.HoldReason != nil && (len(*settings.HoldReason) > 500 || strings.ContainsAny(*settings.HoldReason, "\r\n")) {
		return fmt.Errorf("hold reason exceeds supported bounds")
	}
	if len(settings.DependsOn) > 50 {
		return fmt.Errorf("too many dependencies")
	}
	seen := make(map[int]bool, len(settings.DependsOn))
	for _, dependency := range settings.DependsOn {
		if dependency <= 0 || dependency == number || seen[dependency] {
			return fmt.Errorf("dependency must be a distinct same-repository issue number other than this issue")
		}
		seen[dependency] = true
	}
	return nil
}

// Until project check profiles are configured, only inheritance can be resolved.
// A source name never becomes a command by itself.
func ValidateProfileReference(req Requirements) error {
	if req.Settings.CheckProfile != "inherit" {
		return fmt.Errorf("check profile %q is not configured for this project", req.Settings.CheckProfile)
	}
	return nil
}
