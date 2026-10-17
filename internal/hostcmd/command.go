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

const maxOutput = 2 << 20

type limitedBuffer struct {
	bytes.Buffer
	overflow bool
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
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
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
		return nil, fmt.Errorf("%s command failed; check the tool's login and repository access", name)
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("%s output exceeded limit", name)
	}
	return stdout.Bytes(), nil
}
