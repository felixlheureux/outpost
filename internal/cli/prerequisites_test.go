package cli

import (
	"reflect"
	"testing"
)

func TestPrerequisitesReuseExistingTools(t *testing.T) {
	got := planPrerequisites(func(string) bool { return true })
	if len(got.apt) != 0 || got.tailscale || got.codex {
		t.Fatalf("installed tools were scheduled for replacement: %+v", got)
	}
}

func TestPrerequisitesChooseScopedActions(t *testing.T) {
	installed := map[string]bool{"git": true, "curl": true}
	got := planPrerequisites(func(name string) bool { return installed[name] })
	if !reflect.DeepEqual(got.apt, []string{"gh"}) || !got.tailscale || !got.codex {
		t.Fatalf("missing tools plan = %+v", got)
	}
}
