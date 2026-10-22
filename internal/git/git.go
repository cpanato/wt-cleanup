// Package git wraps the git CLI calls wt-cleanup needs.
//
// It shells out to git rather than using a Go implementation so behaviour
// (config, credential helpers, SSH agents) matches the user's own git.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Repo is a git repository addressed by a directory inside it.
type Repo struct {
	Dir string
}

// Worktree is one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	Branch   string // short name; empty when detached or bare
	Detached bool
	Bare     bool
}

func run(ctx context.Context, dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// runAttached runs git with stdin/stdout/stderr attached to the terminal, for
// commands whose own output or prompts (SSH key touch) the user should see.
func runAttached(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Open resolves dir to the top level of its repository.
func Open(ctx context.Context, dir string) (*Repo, error) {
	top, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repo: %s", dir)
	}
	return &Repo{Dir: top}, nil
}

// Worktrees lists all worktrees; the first entry is the primary one.
func (r *Repo) Worktrees(ctx context.Context) ([]Worktree, error) {
	out, err := run(ctx, r.Dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseWorktrees(out), nil
}

// ParseWorktrees parses `git worktree list --porcelain` output.
func ParseWorktrees(out string) []Worktree {
	var wts []Worktree
	var cur *Worktree
	for line := range strings.SplitSeq(out, "\n") {
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			wts = append(wts, Worktree{Path: val})
			cur = &wts[len(wts)-1]
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		}
	}
	return wts
}

// Fetch runs `git fetch --all --prune` attached to the terminal.
func (r *Repo) Fetch(ctx context.Context) error {
	return runAttached(ctx, r.Dir, "fetch", "--all", "--prune", "--quiet")
}

// RefExists reports whether ref resolves to an object.
func (r *Repo) RefExists(ctx context.Context, ref string) bool {
	_, err := run(ctx, r.Dir, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// PickBase returns the first existing ref among the usual default branches,
// preferring upstream over origin over local. It returns "" if none exist.
func (r *Repo) PickBase(ctx context.Context) string {
	for _, ref := range []string{"upstream/main", "upstream/master", "origin/main", "origin/master", "main", "master"} {
		if r.RefExists(ctx, ref) {
			return ref
		}
	}
	return ""
}

// GitHubNWO returns "owner/repo" for the upstream (preferred) or origin remote.
func (r *Repo) GitHubNWO(ctx context.Context) string {
	for _, remote := range []string{"upstream", "origin"} {
		url, err := run(ctx, r.Dir, "remote", "get-url", remote)
		if err != nil || url == "" {
			continue
		}
		return NWOFromURL(url)
	}
	return ""
}

// NWOFromURL extracts "owner/repo" from an SSH or HTTPS remote URL.
func NWOFromURL(url string) string {
	s := strings.TrimSpace(url)
	switch {
	case strings.HasPrefix(s, "git@"):
		if _, rest, ok := strings.Cut(s, ":"); ok {
			s = rest
		}
	case strings.Contains(s, "://"):
		_, rest, _ := strings.Cut(s, "://")
		if _, path, ok := strings.Cut(rest, "/"); ok {
			s = path
		}
	}
	return strings.TrimSuffix(s, ".git")
}

// LastFetch returns the mtime of FETCH_HEAD, or false if the repo was never fetched.
func (r *Repo) LastFetch(ctx context.Context) (time.Time, bool) {
	dir, err := run(ctx, r.Dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return time.Time{}, false
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(r.Dir, dir)
	}
	fi, err := os.Stat(filepath.Join(dir, "FETCH_HEAD"))
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}

// StashCount returns the number of stash entries (shared by all worktrees).
func (r *Repo) StashCount(ctx context.Context) int {
	out, err := run(ctx, r.Dir, "stash", "list")
	if err != nil {
		return 0
	}
	return countLines(out)
}

// DirtyCount returns the number of `git status --porcelain` entries in dir.
func DirtyCount(ctx context.Context, dir string) (int, error) {
	out, err := run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	return countLines(out), nil
}

// RevCount returns the number of commits in rangeSpec (e.g. "a..b").
func (r *Repo) RevCount(ctx context.Context, rangeSpec string) (int, error) {
	out, err := run(ctx, r.Dir, "rev-list", "--count", rangeSpec)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// Upstream returns the upstream tracking ref of branch, or "" if it has none
// or the remote branch no longer exists.
func (r *Repo) Upstream(ctx context.Context, branch string) string {
	up, err := run(ctx, r.Dir, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil || up == "" || !r.RefExists(ctx, up) {
		return ""
	}
	return up
}

// IsAncestor reports whether a is an ancestor of b.
func (r *Repo) IsAncestor(ctx context.Context, a, b string) bool {
	_, err := run(ctx, r.Dir, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// RemoveWorktree runs `git worktree remove`, showing git's own errors.
func (r *Repo) RemoveWorktree(ctx context.Context, path string) error {
	return runAttached(ctx, r.Dir, "worktree", "remove", path)
}

// DeleteBranch force-deletes a local branch.
func (r *Repo) DeleteBranch(ctx context.Context, branch string) error {
	_, err := run(ctx, r.Dir, "branch", "-D", branch)
	return err
}

// PruneWorktrees runs `git worktree prune`.
func (r *Repo) PruneWorktrees(ctx context.Context) error {
	_, err := run(ctx, r.Dir, "worktree", "prune")
	return err
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
