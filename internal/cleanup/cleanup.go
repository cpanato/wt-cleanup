// Package cleanup runs the classify-then-act flow over one or more repos.
package cleanup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"charm.land/lipgloss/v2"

	"github.com/cpanato/wt-cleanup/internal/classify"
	"github.com/cpanato/wt-cleanup/internal/gh"
	"github.com/cpanato/wt-cleanup/internal/git"
	"github.com/cpanato/wt-cleanup/internal/ui"
)

// Options mirror the command-line flags.
type Options struct {
	Apply  bool
	Yes    bool
	Fetch  bool
	Base   string // override; empty picks a default
	UseGH  bool
	Caches bool
}

// Prompter asks the user to confirm actions. It is swappable for tests.
type Prompter struct {
	AskRemove func(title string) ui.Answer
	Confirm   func(title string) bool
}

// Totals counts verdicts across all repos.
type Totals struct {
	Remove, Keep, Review, Prune int
}

// Runner holds state for one invocation.
type Runner struct {
	Opts   Options
	Out    io.Writer
	Prompt Prompter
	Totals Totals
}

// New returns a Runner writing to stdout with interactive prompts.
func New(opts Options) *Runner {
	return &Runner{
		Opts:   opts,
		Out:    os.Stdout,
		Prompt: Prompter{AskRemove: ui.AskRemove, Confirm: ui.Confirm},
	}
}

func (r *Runner) printf(format string, a ...any) {
	_, _ = lipgloss.Fprintf(r.Out, format, a...)
}

type removal struct {
	path, branch string
}

// Repo processes a single repository. Errors are reported and returned but
// the caller is expected to continue with the next repo.
func (r *Runner) Repo(ctx context.Context, dir string) error {
	repo, err := git.Open(ctx, dir)
	if err != nil {
		r.printf("%s\n", ui.Red.Render(err.Error()))
		return err
	}
	wts, err := repo.Worktrees(ctx)
	if err != nil || len(wts) == 0 {
		r.printf("%s\n", ui.Red.Render(fmt.Sprintf("cannot list worktrees in %s", dir)))
		return err
	}
	// Always drive from the primary worktree so `git worktree remove` works.
	repo = &git.Repo{Dir: wts[0].Path}

	r.printf("\n%s\n", ui.Bold.Render("=== "+repo.Dir+" ==="))

	if r.Opts.Fetch {
		r.printf("%s\n", ui.Dim.Render("fetching all remotes (SSH may ask for a YubiKey touch)..."))
		if err := repo.Fetch(ctx); err != nil {
			r.printf("%s\n", ui.Yellow.Render("fetch failed, using local refs"))
		}
	}

	base := r.Opts.Base
	if base == "" {
		base = repo.PickBase(ctx)
	}
	if base == "" {
		r.printf("%s\n", ui.Red.Render("no usable base ref; skipping"))
		return fmt.Errorf("%s: no usable base ref", repo.Dir)
	}
	nwo := repo.GitHubNWO(ctx)

	fetched := "never"
	if t, ok := repo.LastFetch(ctx); ok {
		fetched = t.Format("2006-01-02 15:04")
	}
	shownNWO := nwo
	if shownNWO == "" {
		shownNWO = "<none>"
	}
	r.printf("%s\n", ui.Dim.Render(fmt.Sprintf("base=%s  github=%s  fetched=%s", base, shownNWO, fetched)))

	if n := repo.StashCount(ctx); n > 0 {
		r.printf("%s\n", ui.Yellow.Render(fmt.Sprintf("note: %d stash entry/entries in this repo (shared by all worktrees) — not touched", n)))
	}

	var lookup classify.PRLookup
	if r.Opts.UseGH && nwo != "" {
		lookup = func(branch string) *gh.PR { return gh.LatestPR(ctx, nwo, branch) }
	}

	var toRemove []removal
	needPrune := false
	for _, wt := range wts[1:] {
		facts := r.gather(ctx, repo, wt, base)
		res := classify.Classify(facts, base, lookup)
		r.printRow(wt, res)

		switch res.Verdict {
		case classify.Prune:
			needPrune = true
			r.Totals.Prune++
		case classify.Remove:
			toRemove = append(toRemove, removal{wt.Path, wt.Branch})
			r.Totals.Remove++
		case classify.Keep:
			r.Totals.Keep++
		default:
			r.Totals.Review++
		}
	}

	if r.Opts.Apply {
		r.apply(ctx, repo, toRemove, needPrune)
	}
	if r.Opts.Caches {
		r.caches(ctx, repo)
	}
	return nil
}

func (r *Runner) gather(ctx context.Context, repo *git.Repo, wt git.Worktree, base string) classify.Facts {
	if _, err := os.Stat(wt.Path); err != nil {
		return classify.Facts{Missing: true}
	}
	f := classify.Facts{Branch: wt.Branch, Ahead: classify.Unknown, Unpushed: classify.Unknown}

	if n, err := git.DirtyCount(ctx, wt.Path); err != nil {
		f.Dirty = classify.Unknown
	} else {
		f.Dirty = n
	}
	if f.Branch == "" {
		return f
	}
	if n, err := repo.RevCount(ctx, base+".."+f.Branch); err == nil {
		f.Ahead = n
	}
	if up := repo.Upstream(ctx, f.Branch); up != "" {
		if n, err := repo.RevCount(ctx, up+".."+f.Branch); err == nil {
			f.Unpushed = n
		}
	}
	f.InBase = repo.IsAncestor(ctx, f.Branch, base)
	return f
}

func (r *Runner) printRow(wt git.Worktree, res classify.Result) {
	name := fmt.Sprintf("%-38s", filepath.Base(wt.Path))
	branch := wt.Branch
	if res.Verdict == classify.Prune {
		branch = "(directory gone)"
	}
	col := ui.Dim.Render(fmt.Sprintf("%-42s", branch))

	var verdict string
	switch res.Verdict {
	case classify.Prune:
		verdict = ui.Yellow.Render(string(res.Verdict))
	case classify.Remove:
		verdict = ui.Green.Render(fmt.Sprintf("%-7s (%s)", res.Verdict, res.Reason))
	case classify.Keep:
		verdict = fmt.Sprintf("%-7s (%s)", res.Verdict, res.Reason)
	default:
		verdict = ui.Yellow.Render(fmt.Sprintf("%-7s (%s)", res.Verdict, res.Reason))
	}
	r.printf("  %s %s %s\n", name, col, verdict)
}

func (r *Runner) apply(ctx context.Context, repo *git.Repo, toRemove []removal, needPrune bool) {
loop:
	for _, rm := range toRemove {
		if !r.Opts.Yes {
			switch r.Prompt.AskRemove(fmt.Sprintf("remove worktree %s and branch %s?", rm.path, rm.branch)) {
			case ui.Quit:
				r.printf("stopping\n")
				break loop
			case ui.No:
				r.printf("  skipped\n")
				continue
			}
		}
		if err := repo.RemoveWorktree(ctx, rm.path); err != nil {
			r.printf("  %s to remove %s — left in place\n", ui.Red.Render("failed"), rm.path)
			continue
		}
		if err := repo.DeleteBranch(ctx, rm.branch); err != nil {
			r.printf("  %s %s (branch %s not deleted: %v)\n", ui.Green.Render("removed"), rm.path, rm.branch, err)
			continue
		}
		r.printf("  %s %s (branch %s deleted)\n", ui.Green.Render("removed"), rm.path, rm.branch)
	}
	if needPrune {
		if err := repo.PruneWorktrees(ctx); err == nil {
			r.printf("  %s stale worktree records\n", ui.Green.Render("pruned"))
		}
	}
}

func (r *Runner) caches(ctx context.Context, repo *git.Repo) {
	r.printf("  %s (regenerate on next terraform init / npm install):\n", ui.Bold.Render("build/provider caches"))

	// Re-list: worktrees removed above should not be scanned.
	wts, err := repo.Worktrees(ctx)
	if err != nil {
		return
	}
	var dirs []string
	for _, wt := range wts {
		if fi, err := os.Stat(wt.Path); err == nil && fi.IsDir() {
			for _, d := range findCaches(wt.Path) {
				// Nested worktrees can be found from their parent too.
				if !slices.Contains(dirs, d) {
					dirs = append(dirs, d)
				}
			}
		}
	}

	for _, d := range dirs {
		r.printf("    %8s  %s\n", humanSize(dirSize(d)), d)
		if !r.Opts.Apply {
			continue
		}
		if !r.Opts.Yes && !r.Prompt.Confirm("delete "+d+"?") {
			r.printf("      kept\n")
			continue
		}
		if err := os.RemoveAll(d); err != nil {
			r.printf("      %s: %v\n", ui.Red.Render("failed"), err)
			continue
		}
		r.printf("      %s\n", ui.Green.Render("deleted"))
	}
}

// Summary prints totals and, on a dry run, how to act.
func (r *Runner) Summary() {
	t := r.Totals
	r.printf("\n%s %s, %d keep (open PR), %s, %d stale record(s)\n",
		ui.Bold.Render("summary:"),
		ui.Green.Render(fmt.Sprintf("%d removable", t.Remove)),
		t.Keep,
		ui.Yellow.Render(fmt.Sprintf("%d to review", t.Review)),
		t.Prune)
	if !r.Opts.Apply {
		r.printf("%s\n", ui.Dim.Render("dry run — re-run with --apply (add --yes to skip prompts) to act"))
	}
}
