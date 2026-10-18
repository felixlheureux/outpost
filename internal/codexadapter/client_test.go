package codexadapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("OUTPOST_CODEX_HELPER"); mode != "" {
		fixture(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fixture(mode string) {
	r := bufio.NewScanner(os.Stdin)
	for r.Scan() {
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(r.Bytes(), &request) != nil || request.ID == 0 {
			continue
		}
		if mode == "oversize" {
			fmt.Println(strings.Repeat("x", MaxFrame+1))
			return
		}
		if mode == "reject" && request.Method == "thread/resume" {
			fmt.Printf("{\"id\":%d,\"error\":{\"code\":404,\"message\":\"session missing\"}}\n", request.ID)
			continue
		}
		result := "{}"
		switch request.Method {
		case "model/list":
			result = `{"data":[{"model":"fixture","displayName":"Fixture","supportedReasoningEfforts":[{"reasoningEffort":"medium"}]}],"nextCursor":null}`
		case "thread/start", "thread/resume":
			result = `{"thread":{"id":"thread-1","cwd":"/tmp"}}`
		case "turn/start":
			result = `{"turn":{"id":"turn-1"}}`
		case "turn/steer":
			result = `{"turnId":"turn-1"}`
		}
		fmt.Printf("{\"id\":%d,\"result\":%s}\n", request.ID, result)
		if request.Method == "turn/start" && mode == "success" {
			fmt.Println(`{"method":"item/completed","params":{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","text":"{\"summary\":\"done\",\"changed_paths\":[],\"claimed_checks\":[],\"blockers\":[]}"}}}`)
			fmt.Println(`{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}}`)
		}
	}
}

func TestProtocolFixture(t *testing.T) {
	t.Setenv("OUTPOST_CODEX_HELPER", "success")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Start(ctx, os.Args[0], t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	models, err := c.Models(ctx)
	if err != nil || len(models) != 1 || models[0].ID != "fixture" {
		t.Fatalf("models %+v: %v", models, err)
	}
	thread, err := c.Thread(ctx, Settings{Workspace: "/tmp", Model: "fixture", Reasoning: "medium", Sandbox: "workspace-write", ApprovalPolicy: "on-request"})
	if err != nil || thread != "thread-1" {
		t.Fatalf("thread %s: %v", thread, err)
	}
	turn, err := c.StartTurn(ctx, thread, "test", "medium", HandoffSchema)
	if err != nil || turn != "turn-1" {
		t.Fatalf("turn %s: %v", turn, err)
	}
	result, err := c.WaitTurn(ctx, thread, turn, nil)
	if err != nil || result.Handoff.Summary != "done" {
		t.Fatalf("result %+v: %v", result, err)
	}
	if err := c.Steer(ctx, thread, turn, "more"); err != nil {
		t.Fatal(err)
	}
	if err := c.Interrupt(ctx, thread, turn); err != nil {
		t.Fatal(err)
	}
}

func TestResumeRejectionAndOversizeFrame(t *testing.T) {
	for _, mode := range []string{"reject", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("OUTPOST_CODEX_HELPER", mode)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			c, err := Start(ctx, os.Args[0], t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if mode == "reject" {
				if err := c.Initialize(ctx); err != nil {
					t.Fatal(err)
				}
				_, err = c.Thread(ctx, Settings{Workspace: "/tmp", Model: "fixture", Reasoning: "medium", Sandbox: "workspace-write", ApprovalPolicy: "on-request", ThreadID: "missing"})
				if err == nil || !strings.Contains(err.Error(), "session missing") {
					t.Fatalf("resume: %v", err)
				}
			} else {
				if err := c.Initialize(ctx); err == nil {
					t.Fatal("oversized frame accepted")
				}
			}
		})
	}
}
