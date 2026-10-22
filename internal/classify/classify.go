// Package classify decides what to do with a worktree. It is pure so the
// safety rules can be tested without git or GitHub.
package classify

import (
	"fmt"
	"strconv"

	"github.com/cpanato/wt-cleanup/internal/gh"
)

// Verdict is the action for a worktree.
type Verdict string

const (
	// Prune means the directory is gone and only the admin record remains.
	Prune Verdict = "PRUNE"
	// Remove means the work has landed and nothing would be lost.
	Remove Verdict = "REMOVE"
	// Keep means the branch has an open PR.
	Keep Verdict = "KEEP"
	// Review means a human should look before anything is deleted.
	Review Verdict = "REVIEW"
)

// Unknown marks a count that could not be determined.
const Unknown = -1

// Facts is what we know about a worktree.
type Facts struct {
	Missing  bool   // directory no longer exists
	Branch   string // empty when detached
	Dirty    int    // uncommitted entries; Unknown if git status failed
	Ahead    int    // commits not in base; Unknown if not computable
	Unpushed int    // commits not on the upstream branch; Unknown if no live upstream
	InBase   bool   // branch is an ancestor of base
}

// Result is a verdict plus a human-readable reason.
type Result struct {
	Verdict Verdict
	Reason  string
}

// PRLookup returns the newest PR for a branch, or nil. It is only called
// when the ancestor test is not conclusive.
type PRLookup func(branch string) *gh.PR

// Classify applies the cleanup rules to f.
func Classify(f Facts, base string, lookup PRLookup) Result {
	if f.Missing {
		return Result{Prune, "directory gone"}
	}

	r := Result{Verdict: Review}
	switch {
	case f.Branch == "":
		r.Reason = "detached HEAD"
	case f.Dirty == Unknown:
		r.Reason = "git status failed"
	case f.Dirty != 0:
		r.Reason = fmt.Sprintf("%d uncommitted change(s)", f.Dirty)
	case f.InBase:
		r = Result{Remove, "in " + base}
	default:
		var pr *gh.PR
		if lookup != nil {
			pr = lookup(f.Branch)
		}
		switch {
		case pr != nil && pr.State == "MERGED":
			r = Result{Remove, fmt.Sprintf("PR #%d merged (squash/rebase)", pr.Number)}
		case pr != nil && pr.State == "CLOSED":
			r = Result{Remove, fmt.Sprintf("PR #%d closed unmerged", pr.Number)}
		case pr != nil && pr.State == "OPEN":
			r = Result{Keep, fmt.Sprintf("PR #%d open", pr.Number)}
		default:
			r.Reason = fmt.Sprintf("no PR, %s commit(s) not in %s, unpushed=%s", count(f.Ahead), base, count(f.Unpushed))
		}
	}

	// Never drop commits that exist only here.
	if r.Verdict == Remove && f.Unpushed > 0 {
		r = Result{Review, fmt.Sprintf("%s but %d unpushed commit(s)", r.Reason, f.Unpushed)}
	}
	return r
}

func count(n int) string {
	if n == Unknown {
		return "?"
	}
	return strconv.Itoa(n)
}
