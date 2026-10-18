// Package hostcmd is the bounded, noninteractive process boundary for trusted host tools.
package hostcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maxOutput = 16 << 20

type limitedBuffer struct {
	bytes.Buffer
	overflow bool
}

// CommandError exposes only a diagnostic category. Raw CLI output may contain credentials.
type CommandError struct{ Kind string }

func (e *CommandError) Error() string {
	return "host command " + e.Kind + "; check the tool's login and repository access"
}

func classifyFailure(stderr string) string {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "http 409"):
		return "conflict"
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "http 429"), strings.Contains(lower, "secondary rate"):
		return "rate_limited"
	case strings.Contains(lower, "http 401"), strings.Contains(lower, "authentication required"), strings.Contains(lower, "not logged in"):
		return "authentication_required"
	case strings.Contains(lower, "http 403"), strings.Contains(lower, "resource not accessible"), strings.Contains(lower, "permission"):
		return "permission_denied"
	case strings.Contains(lower, "http 404"), strings.Contains(lower, "not found"):
		return "inaccessible_or_missing"
	case strings.Contains(lower, "http 5"), strings.Contains(lower, "timed out"), strings.Contains(lower, "connection"):
		return "unavailable"
	default:
		return "failed"
	}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxOutput - b.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.overflow = true
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

// Run receives argv, never a shell string. Outputs/errors must not be logged as credentials may occur in tool output.
func Run(ctx context.Context, dir, name string, input io.Reader, args ...string) ([]byte, error) {
	return RunWithTimeout(ctx, 20*time.Second, dir, name, input, args...)
}

func RunWithTimeout(ctx context.Context, timeout time.Duration, dir, name string, input io.Reader, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Stdin = dir, input
	cmd.WaitDelay = 2 * time.Second
	// Disable prompts, browser launch, paging and verbose auth debugging. Preserve the user's normal credential environment.
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		switch key {
		case "GH_DEBUG", "GH_PROMPT_DISABLED", "GH_PAGER", "GH_BROWSER", "GIT_TERMINAL_PROMPT", "GIT_TRACE", "GIT_TRACE_CURL", "GIT_CURL_VERBOSE":
			continue
		}
		cmd.Env = append(cmd.Env, v)
	}
	cmd.Env = append(cmd.Env, "GH_PROMPT_DISABLED=1", "GH_PAGER=cat", "GH_BROWSER=false", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr limitedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s did not complete: %w", name, ctx.Err())
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("%s is not installed", name)
		}
		return nil, &CommandError{Kind: classifyFailure(stderr.String())}
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("%s output exceeded limit", name)
	}
	return stdout.Bytes(), nil
}
