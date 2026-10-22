package cleanup

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpanato/wt-cleanup/internal/ui"
)

// sandbox builds an origin + clone with one worktree per verdict.
type sandbox struct {
	t    *testing.T
	root string
	repo string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(root, "gitconfig")
	if err := os.WriteFile(cfg, []byte("[user]\n\tname = test\n\temail = test@example.com\n[init]\n\tdefaultBranch = main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	s := &sandbox{t: t, root: root, repo: filepath.Join(root, "repo")}
	s.git(root, "init", "--bare", "-q", "origin.git")
	s.git(root, "clone", "-q", filepath.Join(root, "origin.git"), "repo")
	s.commit(s.repo, "init")
	s.git(s.repo, "push", "-q", "-u", "origin", "main")
	return s
}

func (s *sandbox) git(dir string, args ...string) string {
	s.t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		s.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (s *sandbox) commit(dir, msg string) {
	s.t.Helper()
	s.git(dir, "commit", "-q", "--allow-empty", "-m", msg)
}

func (s *sandbox) worktree(name string) string {
	s.t.Helper()
	p := filepath.Join(s.root, name)
	s.git(s.repo, "worktree", "add", "-q", "-b", name, p, "main")
	return p
}

func (s *sandbox) exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (s *sandbox) hasBranch(name string) bool {
	return exec.Command("git", "-C", s.repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+name).Run() == nil
}

func setup(t *testing.T) (*sandbox, map[string]string) {
	s := newSandbox(t)
	wt := map[string]string{}

	// Merged into origin/main and pushed.
	wt["merged"] = s.worktree("merged")
	s.commit(wt["merged"], "merged work")
	s.git(s.repo, "merge", "-q", "--ff-only", "merged")
	s.git(s.repo, "push", "-q", "origin", "main")

	// In base, but the branch's own upstream lags behind it.
	wt["unpushed"] = s.worktree("unpushed")
	s.commit(wt["unpushed"], "pushed")
	s.git(wt["unpushed"], "push", "-q", "-u", "origin", "unpushed")
	s.commit(wt["unpushed"], "local only")
	s.git(s.repo, "merge", "-q", "--ff-only", "unpushed")
	s.git(s.repo, "push", "-q", "origin", "main")

	// Not merged, no PR.
	wt["wip"] = s.worktree("wip")
	s.commit(wt["wip"], "wip")

	// Uncommitted changes.
	wt["dirty"] = s.worktree("dirty")
	if err := os.WriteFile(filepath.Join(wt["dirty"], "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Detached HEAD.
	wt["detached"] = filepath.Join(s.root, "detached")
	s.git(s.repo, "worktree", "add", "-q", "--detach", wt["detached"], "main")

	// Directory deleted behind git's back.
	wt["gone"] = s.worktree("gone")
	if err := os.RemoveAll(wt["gone"]); err != nil {
		t.Fatal(err)
	}

	// Caches in a surviving tree.
	if err := os.MkdirAll(filepath.Join(wt["wip"], "infra", ".terraform", "providers"), 0o755); err != nil {
		t.Fatal(err)
	}
	return s, wt
}

func run(t *testing.T, opts Options, dir string, prompt Prompter) (*Runner, string) {
	t.Helper()
	var buf bytes.Buffer
	r := &Runner{Opts: opts, Out: &buf, Prompt: prompt}
	if err := r.Repo(context.Background(), dir); err != nil {
		t.Fatalf("Repo: %v\n%s", err, buf.String())
	}
	r.Summary()
	return r, buf.String()
}

func TestDryRun(t *testing.T) {
	s, wt := setup(t)

	// Run from inside a linked worktree to check we resolve the primary.
	r, out := run(t, Options{Caches: true}, wt["wip"], Prompter{})

	for _, want := range []string{
		"=== " + s.repo + " ===",
		"base=origin/main",
		"REMOVE  (in origin/main)",
		"REVIEW  (in origin/main but 1 unpushed commit(s))",
		"REVIEW  (no PR, 1 commit(s) not in origin/main, unpushed=?)",
		"REVIEW  (1 uncommitted change(s))",
		"REVIEW  (detached HEAD)",
		"(directory gone)",
		filepath.Join(wt["wip"], "infra", ".terraform"),
		"dry run",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if want := (Totals{Remove: 1, Review: 4, Prune: 1}); r.Totals != want {
		t.Errorf("totals = %+v, want %+v", r.Totals, want)
	}
	for name, p := range wt {
		if name != "gone" && !s.exists(p) {
			t.Errorf("dry run removed %s", name)
		}
	}
}

func TestApplyYes(t *testing.T) {
	s, wt := setup(t)
	_, out := run(t, Options{Apply: true, Yes: true, Caches: true}, s.repo, Prompter{})

	if s.exists(wt["merged"]) || s.hasBranch("merged") {
		t.Errorf("merged worktree/branch should be removed\n%s", out)
	}
	for _, name := range []string{"unpushed", "wip", "dirty"} {
		if !s.exists(wt[name]) || !s.hasBranch(name) {
			t.Errorf("%s should be left alone\n%s", name, out)
		}
	}
	if strings.Contains(s.git(s.repo, "worktree", "list", "--porcelain"), wt["gone"]) {
		t.Errorf("stale record for gone worktree should be pruned\n%s", out)
	}
	if s.exists(filepath.Join(wt["wip"], "infra", ".terraform")) {
		t.Errorf(".terraform cache should be deleted\n%s", out)
	}
}

func TestApplyPrompts(t *testing.T) {
	s, wt := setup(t)
	var asked []string
	prompt := Prompter{
		AskRemove: func(title string) ui.Answer { asked = append(asked, title); return ui.No },
		Confirm:   func(string) bool { return false },
	}
	_, out := run(t, Options{Apply: true, Caches: true}, s.repo, prompt)

	if len(asked) != 1 || !strings.Contains(asked[0], wt["merged"]) {
		t.Errorf("asked = %q, want one prompt for merged", asked)
	}
	if !s.exists(wt["merged"]) || !strings.Contains(out, "skipped") {
		t.Errorf("answering no should skip\n%s", out)
	}
	if !s.exists(filepath.Join(wt["wip"], "infra", ".terraform")) || !strings.Contains(out, "kept") {
		t.Errorf("declined cache should be kept\n%s", out)
	}
}

func TestBaseOverrideAndNotARepo(t *testing.T) {
	s, _ := setup(t)
	_, out := run(t, Options{Base: "main"}, s.repo, Prompter{})
	if !strings.Contains(out, "base=main ") {
		t.Errorf("--base not honoured\n%s", out)
	}

	var buf bytes.Buffer
	r := &Runner{Out: &buf}
	if err := r.Repo(context.Background(), t.TempDir()); err == nil || !strings.Contains(buf.String(), "not a git repo") {
		t.Errorf("want not-a-repo error, got %v\n%s", err, buf.String())
	}
}

func TestFindCaches(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{
		"node_modules/pkg/node_modules", // nested: only the outer one counts
		"a/b/c/.terraform",              // depth 4: found
		"a/b/c/d/node_modules",          // depth 5: too deep
	} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := findCaches(root)
	want := []string{filepath.Join(root, "a/b/c/.terraform"), filepath.Join(root, "node_modules")}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("findCaches() = %v, want %v", got, want)
	}
}

func TestHumanSize(t *testing.T) {
	for n, want := range map[int64]string{0: "0B", 1023: "1023B", 1024: "1.0K", 1536: "1.5K", 5 << 20: "5.0M", 3 << 30: "3.0G"} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}
