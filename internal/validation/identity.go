package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// CandidateIdentity includes index paths, their working-tree bytes, and relevant
// untracked files. Git metadata, issue display fields, and commit identity are not
// part of a code check's input. Ignored inputs must be declared by the profile.
func CandidateIdentity(ctx context.Context, workspace string, supplemental []string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("list candidate inputs: %w", err)
	}
	if len(b) > 16<<20 {
		return "", fmt.Errorf("candidate path list exceeds limit")
	}
	seen := map[string]bool{}
	for _, v := range strings.Split(string(b), "\x00") {
		if v != "" {
			seen[v] = true
		}
	}
	for _, pattern := range supplemental {
		matches, err := filepath.Glob(filepath.Join(workspace, pattern))
		if err != nil {
			return "", err
		}
		for _, match := range matches {
			rel, err := filepath.Rel(workspace, match)
			if err != nil || !safeRelative(rel) {
				return "", fmt.Errorf("supplemental input escapes workspace")
			}
			seen[filepath.ToSlash(rel)] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	h := sha256.New()
	for _, p := range paths {
		if !safeRelative(p) {
			return "", fmt.Errorf("unsafe candidate path")
		}
		full := filepath.Join(workspace, filepath.FromSlash(p))
		info, err := os.Lstat(full)
		fmt.Fprintf(h, "%d:%s\x00", len(p), p)
		if os.IsNotExist(err) {
			io.WriteString(h, "deleted\x00")
			continue
		}
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%o\x00", info.Mode().Type()|info.Mode().Perm())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(full)
			if err != nil {
				return "", err
			}
			io.WriteString(h, target)
			continue
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("candidate input %q is not a regular file", p)
		}
		if info.Size() > 64<<20 {
			return "", fmt.Errorf("candidate input %q exceeds 64 MiB", p)
		}
		f, err := os.Open(full)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
