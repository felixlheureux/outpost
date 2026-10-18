package githubcli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/felixlheureux/outpost/internal/hostcmd"
)

const QueueBranch = "outpost/queue"
const QueuePath = "queue.json"

type QueueFile struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Type     string `json:"type"`
}

func queueEndpoint(owner, name string) (string, error) {
	if !part.MatchString(owner) || !part.MatchString(name) {
		return "", fmt.Errorf("invalid repository identity")
	}
	return "repos/" + owner + "/" + name, nil
}

func ReadQueue(ctx context.Context, owner, name string) ([]byte, string, error) {
	base, err := queueEndpoint(owner, name)
	if err != nil {
		return nil, "", err
	}
	b, err := readAPI(ctx, base+"/contents/"+QueuePath+"?ref="+url.QueryEscape(QueueBranch))
	if err != nil {
		return nil, "", err
	}
	var file QueueFile
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, "", fmt.Errorf("decode planning file: %w", err)
	}
	if file.Type != "file" || file.Encoding != "base64" || file.SHA == "" || len(file.Content) > 400000 {
		return nil, "", fmt.Errorf("planning file metadata is invalid")
	}
	content, err := base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, file.Content))
	if err != nil || len(content) > 256<<10 {
		return nil, "", fmt.Errorf("planning file content is invalid or oversized")
	}
	return content, file.SHA, nil
}

func WriteQueue(ctx context.Context, owner, name, sha, operationID string, content []byte) (string, error) {
	base, err := queueEndpoint(owner, name)
	if err != nil {
		return "", err
	}
	if sha == "" || operationID == "" || len(content) > 256<<10 {
		return "", fmt.Errorf("invalid planning write")
	}
	payload, _ := json.Marshal(map[string]string{"message": "outpost queue operation " + operationID, "content": base64.StdEncoding.EncodeToString(content), "branch": QueueBranch, "sha": sha})
	b, err := hostcmd.Run(ctx, "", "gh", bytes.NewReader(payload), "api", "--hostname", "github.com", "--method", "PUT", "--input", "-", base+"/contents/"+QueuePath)
	if err != nil {
		return "", err
	}
	var result struct {
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
	}
	if err := json.Unmarshal(b, &result); err != nil || result.Content.SHA == "" {
		return "", fmt.Errorf("planning write response is incomplete")
	}
	return result.Content.SHA, nil
}

// EnsureQueue creates only missing source objects. A read error other than an
// explicit 404 is never interpreted as absence.
func EnsureQueue(ctx context.Context, owner, name, defaultBranch string) error {
	base, err := queueEndpoint(owner, name)
	if err != nil {
		return err
	}
	if !part.MatchString(defaultBranch) {
		return fmt.Errorf("invalid default branch")
	}
	_, err = readAPI(ctx, base+"/git/ref/heads/"+QueueBranch)
	if err != nil {
		if !isMissing(err) {
			return fmt.Errorf("planning branch access: %w", err)
		}
		b, baseErr := readAPI(ctx, base+"/git/ref/heads/"+defaultBranch)
		if baseErr != nil {
			return fmt.Errorf("default branch access: %w", baseErr)
		}
		var ref struct {
			Object struct {
				SHA string `json:"sha"`
			} `json:"object"`
		}
		if json.Unmarshal(b, &ref) != nil || ref.Object.SHA == "" {
			return fmt.Errorf("default branch has no readable commit")
		}
		payload, _ := json.Marshal(map[string]string{"ref": "refs/heads/" + QueueBranch, "sha": ref.Object.SHA})
		if _, err := hostcmd.Run(ctx, "", "gh", bytes.NewReader(payload), "api", "--hostname", "github.com", "--method", "POST", "--input", "-", base+"/git/refs"); err != nil {
			// A concurrent creator can win. Re-read before treating that as failure.
			if _, readErr := readAPI(ctx, base+"/git/ref/heads/"+QueueBranch); readErr != nil {
				return fmt.Errorf("create planning branch: %w", err)
			}
		}
	}
	_, _, err = ReadQueue(ctx, owner, name)
	if err == nil {
		return nil
	}
	if !isMissing(err) {
		return fmt.Errorf("planning file needs attention: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{"message": "Initialize Outpost queue", "content": base64.StdEncoding.EncodeToString([]byte("{\n  \"version\": 1,\n  \"issues\": []\n}\n")), "branch": QueueBranch})
	if _, err := hostcmd.Run(ctx, "", "gh", bytes.NewReader(payload), "api", "--hostname", "github.com", "--method", "PUT", "--input", "-", base+"/contents/"+QueuePath); err != nil {
		if _, _, readErr := ReadQueue(ctx, owner, name); readErr != nil {
			return fmt.Errorf("initialize planning file: %w", err)
		}
	}
	return nil
}

func isMissing(err error) bool {
	var command *hostcmd.CommandError
	return errors.As(err, &command) && command.Kind == "inaccessible_or_missing"
}

func LatestQueueCommitMessage(ctx context.Context, owner, name string) (string, error) {
	base, err := queueEndpoint(owner, name)
	if err != nil {
		return "", err
	}
	b, err := readAPI(ctx, base+"/commits?sha="+url.QueryEscape(QueueBranch)+"&path="+QueuePath+"&per_page=1")
	if err != nil {
		return "", err
	}
	var commits []struct {
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(b, &commits); err != nil || len(commits) != 1 {
		return "", fmt.Errorf("planning commit could not be inspected")
	}
	return commits[0].Commit.Message, nil
}
