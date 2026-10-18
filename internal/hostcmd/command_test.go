package hostcmd

import "testing"

func TestClassifyFailureRedactsCommandOutput(t *testing.T) {
	for _, test := range []struct{ stderr, kind string }{
		{"HTTP 401: token abc", "authentication_required"},
		{"HTTP 403: Resource not accessible by integration", "permission_denied"},
		{"HTTP 403: API rate limit exceeded", "rate_limited"},
		{"HTTP 429: slow down", "rate_limited"},
		{"HTTP 404: Not Found", "inaccessible_or_missing"},
		{"HTTP 502: server error", "unavailable"},
	} {
		if got := classifyFailure(test.stderr); got != test.kind {
			t.Errorf("classify %q = %q, want %q", test.stderr, got, test.kind)
		}
	}
}
