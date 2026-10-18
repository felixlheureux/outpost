// Package codexadapter speaks the Codex 0.163 app-server stdio protocol.
// It owns only provider transport and identities; Outpost owns admission,
// validation, delivery, and durable receipts.
package codexadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const MaxFrame = 1 << 20
const MaxStderr = 32 << 10

var ErrProtocol = errors.New("Codex app-server protocol error")
var ErrUnknown = errors.New("Codex outcome is unknown; inspect the saved session before retrying")

type frame struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type Event struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type Model struct {
	ID                     string `json:"model"`
	DisplayName            string `json:"displayName"`
	Hidden                 bool   `json:"hidden"`
	Default                bool   `json:"isDefault"`
	DefaultReasoningEffort string `json:"defaultReasoningEffort"`
	ReasoningEfforts       []struct {
		Effort string `json:"reasoningEffort"`
	} `json:"supportedReasoningEfforts"`
}

type Settings struct {
	Workspace      string
	Model          string
	Reasoning      string
	Sandbox        string
	ApprovalPolicy string
	ThreadID       string // Nonempty means explicit resume, never fresh fallback.
}

// Client is one owned app-server process. Calls may run while a turn streams.
// The event callback must not block; durable event writing is the caller's job.
type Client struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	writeMu   sync.Mutex
	mu        sync.Mutex
	nextID    int
	pending   map[int]chan frame
	events    chan Event
	done      chan struct{}
	readErr   error
	stop      context.CancelFunc
	stderr    *boundedTail
	closeOnce sync.Once
}

type boundedTail struct {
	mu sync.Mutex
	b  []byte
}

func (t *boundedTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > MaxStderr {
		t.b = append([]byte(nil), t.b[len(t.b)-MaxStderr:]...)
	}
	return len(p), nil
}

func (t *boundedTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.b)
}

func Start(ctx context.Context, executable, workspace string) (*Client, error) {
	if executable == "" || workspace == "" {
		return nil, fmt.Errorf("Codex executable and workspace are required")
	}
	processCtx, stop := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, executable, "app-server", "--listen", "stdio://")
	cmd.Dir = workspace
	stdin, err := cmd.StdinPipe()
	if err != nil {
		stop()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stop()
		return nil, err
	}
	c := &Client{cmd: cmd, stdin: stdin, pending: map[int]chan frame{}, events: make(chan Event, 128), done: make(chan struct{}), stop: stop, stderr: &boundedTail{}}
	cmd.Stderr = c.stderr
	if err := cmd.Start(); err != nil {
		stop()
		return nil, fmt.Errorf("start Codex app-server: %w", err)
	}
	go c.readLoop(stdout)
	return c, nil
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > MaxFrame {
			return nil, fmt.Errorf("%w: frame exceeds %d bytes", ErrProtocol, MaxFrame)
		}
		line = append(line, part...)
		if err == nil {
			return bytes.TrimSpace(line), nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func (c *Client) readLoop(stdout io.ReadCloser) {
	defer stdout.Close()
	defer close(c.done)
	r := bufio.NewReaderSize(stdout, 64<<10)
	for {
		line, err := readFrame(r)
		if err != nil {
			c.setReadError(err)
			return
		}
		var f frame
		if err := json.Unmarshal(line, &f); err != nil {
			c.setReadError(fmt.Errorf("%w: malformed JSON", ErrProtocol))
			return
		}
		if len(f.ID) > 0 {
			if f.Method != "" { // Server requests never grant permissions without operator control.
				_ = c.send(map[string]any{"id": f.ID, "error": map[string]any{"code": -32601, "message": "Outpost cannot approve provider requests during this task"}})
				continue
			}
			var id int
			if err := json.Unmarshal(f.ID, &id); err != nil {
				c.setReadError(fmt.Errorf("%w: invalid response id", ErrProtocol))
				return
			}
			c.mu.Lock()
			ch := c.pending[id]
			delete(c.pending, id)
			c.mu.Unlock()
			if ch != nil {
				ch <- f
			}
			continue
		}
		if f.Method == "" {
			c.setReadError(fmt.Errorf("%w: unrecognized message", ErrProtocol))
			return
		}
		select {
		case c.events <- Event{Method: f.Method, Params: f.Params}:
		default:
			c.setReadError(fmt.Errorf("%w: event consumer is too slow", ErrProtocol))
			return
		}
	}
}

func (c *Client) setReadError(err error) {
	c.mu.Lock()
	c.readErr = err
	c.mu.Unlock()
	c.stop()
}

func (c *Client) readerError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readErr
}

func (c *Client) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) >= MaxFrame {
		return fmt.Errorf("%w: outgoing frame too large", ErrProtocol)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.stdin.Write(append(b, '\n'))
	return err
}

func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan frame, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.send(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return fmt.Errorf("%w: request write: %v", ErrUnknown, err)
	}
	select {
	case f := <-ch:
		if f.Error != nil {
			return fmt.Errorf("Codex %s rejected request (code %d): %s", method, f.Error.Code, cleanError(f.Error.Message))
		}
		if len(f.Result) == 0 {
			return fmt.Errorf("%w: missing %s result", ErrProtocol, method)
		}
		if result == nil {
			return nil
		}
		if err := json.Unmarshal(f.Result, result); err != nil {
			return fmt.Errorf("%w: invalid %s result", ErrProtocol, method)
		}
		return nil
	case <-c.done:
		return fmt.Errorf("%w: app-server exited before %s acknowledgement: %v", ErrUnknown, method, c.readerError())
	case <-ctx.Done():
		return fmt.Errorf("%w: %s acknowledgement timed out: %v", ErrUnknown, method, ctx.Err())
	}
}

func cleanError(s string) string {
	if len(s) > 300 {
		s = s[:300]
	}
	return strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		return r
	}, s)
}

func (c *Client) Initialize(ctx context.Context) error {
	var result struct {
		UserAgent string `json:"userAgent"`
	}
	if err := c.Call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": "outpost", "title": "Outpost", "version": "0.1.0"}}, &result); err != nil {
		return err
	}
	return c.send(map[string]any{"method": "initialized", "params": map[string]any{}})
}

func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var result struct {
		Data       []Model `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := c.Call(ctx, "model/list", map[string]any{"limit": 100, "includeHidden": false}, &result); err != nil {
		return nil, err
	}
	if result.NextCursor != nil {
		return nil, fmt.Errorf("%w: model catalog exceeds first page", ErrProtocol)
	}
	return result.Data, nil
}

func (c *Client) Thread(ctx context.Context, s Settings) (string, error) {
	if s.Workspace == "" || s.Model == "" || s.Sandbox == "" || s.ApprovalPolicy == "" {
		return "", fmt.Errorf("provider settings are incomplete")
	}
	method := "thread/start"
	params := map[string]any{"cwd": s.Workspace, "model": s.Model, "sandbox": s.Sandbox, "approvalPolicy": s.ApprovalPolicy, "serviceName": "outpost"}
	if s.ThreadID != "" {
		method = "thread/resume"
		params = map[string]any{"threadId": s.ThreadID, "cwd": s.Workspace, "model": s.Model, "sandbox": s.Sandbox, "approvalPolicy": s.ApprovalPolicy, "excludeTurns": true}
	}
	var result struct {
		Thread struct {
			ID  string `json:"id"`
			CWD string `json:"cwd"`
		} `json:"thread"`
	}
	if err := c.Call(ctx, method, params, &result); err != nil {
		return "", err
	}
	if result.Thread.ID == "" || (s.ThreadID != "" && result.Thread.ID != s.ThreadID) || (result.Thread.CWD != "" && result.Thread.CWD != s.Workspace) {
		return "", fmt.Errorf("%w: thread identity or workspace differs", ErrProtocol)
	}
	return result.Thread.ID, nil
}

func (c *Client) StartTurn(ctx context.Context, threadID, prompt, effort string, schema any) (string, error) {
	var result struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	params := map[string]any{"threadId": threadID, "input": []map[string]string{{"type": "text", "text": prompt}}, "effort": effort, "outputSchema": schema}
	if err := c.Call(ctx, "turn/start", params, &result); err != nil {
		return "", err
	}
	if result.Turn.ID == "" {
		return "", fmt.Errorf("%w: missing turn id", ErrProtocol)
	}
	return result.Turn.ID, nil
}

func (c *Client) Steer(ctx context.Context, threadID, turnID, text string) error {
	var result struct {
		TurnID string `json:"turnId"`
	}
	if err := c.Call(ctx, "turn/steer", map[string]any{"threadId": threadID, "expectedTurnId": turnID, "input": []map[string]string{{"type": "text", "text": text}}}, &result); err != nil {
		return err
	}
	if result.TurnID != turnID {
		return fmt.Errorf("%w: steering acknowledgement targeted another turn", ErrProtocol)
	}
	return nil
}

func (c *Client) Interrupt(ctx context.Context, threadID, turnID string) error {
	return c.Call(ctx, "turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil)
}

func (c *Client) Events() <-chan Event { return c.events }
func (c *Client) StderrTail() string   { return c.stderr.String() }

func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.stop()
		_ = c.stdin.Close()
		wait := make(chan error, 1)
		go func() { wait <- c.cmd.Wait() }()
		select {
		case err = <-wait:
		case <-time.After(3 * time.Second):
			_ = c.cmd.Process.Kill()
			err = <-wait
		}
	})
	return err
}
