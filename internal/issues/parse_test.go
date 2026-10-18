package issues

import (
	"strings"
	"testing"
)

func TestParseOrdinaryAndTemplateIssues(t *testing.T) {
	ordinary, err := Parse(12, "Fix the editor", "Keep a draft when navigating.")
	if err != nil || ordinary.Outcome != "Keep a draft when navigating." || ordinary.HasBlock {
		t.Fatalf("ordinary issue = %+v, %v", ordinary, err)
	}
	body := "## Outcome\nKeep the draft.\n\n## Acceptance criteria\n- [ ] Navigation preserves the draft.\n- A rejected save leaves it intact.\n\n## Implementation notes\nUse the existing editor.\n\n## Outpost settings\n```yaml\noutpost_task: 1\nkind: implementation\ndepends_on: [11]\nhold_reason: null\ncheck_profile: inherit\ndelivery: inherit\nreview: inherit\n```\n"
	req, err := Parse(12, "Draft", body)
	if err != nil || req.Outcome != "Keep the draft." || len(req.Criteria) != 2 || req.Criteria[0] != "Navigation preserves the draft." || len(req.Settings.DependsOn) != 1 || !req.HasBlock {
		t.Fatalf("template issue = %+v, %v", req, err)
	}
}

func TestParseRejectsUnsafeSettings(t *testing.T) {
	base := "## Outpost settings\n```yaml\noutpost_task: 1\nkind: implementation\ndepends_on: []\ncheck_profile: inherit\ndelivery: inherit\nreview: inherit\n```\n"
	for name, body := range map[string]string{
		"duplicate block":      base + base,
		"self dependency":      strings.Replace(base, "depends_on: []", "depends_on: [12]", 1),
		"duplicate dependency": strings.Replace(base, "depends_on: []", "depends_on: [3, 3]", 1),
		"unknown key":          strings.Replace(base, "review: inherit", "review: inherit\ncommand: rm -rf .", 1),
		"unsafe policy":        strings.Replace(base, "delivery: inherit", "delivery: shell:$(whoami)", 1),
		"duplicate YAML key":   strings.Replace(base, "delivery: inherit", "delivery: inherit\ndelivery: push", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(12, "Task", body); err == nil {
				t.Fatal("accepted invalid issue settings")
			}
		})
	}
}

func TestParseGitHubIssueFormHeadings(t *testing.T) {
	req, err := Parse(42, "Improve draft", "### Outcome\nKeep text intact.\n\n### Acceptance criteria\n- [ ] Navigation retains text.\n")
	if err != nil || req.Outcome != "Keep text intact." || len(req.Criteria) != 1 {
		t.Fatalf("GitHub form issue = %+v, %v", req, err)
	}
}

func TestUnconfiguredCheckProfileIsNotExecutable(t *testing.T) {
	req, err := Parse(42, "Task", "## Outpost settings\n```yaml\noutpost_task: 1\nkind: implementation\ncheck_profile: shell_command\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateProfileReference(req); err == nil {
		t.Fatal("accepted unconfigured check profile")
	}
}
