package classify

import (
	"testing"

	"github.com/cpanato/wt-cleanup/internal/gh"
)

func prOf(state string, n int) PRLookup {
	return func(string) *gh.PR { return &gh.PR{Number: n, State: state} }
}

func TestClassify(t *testing.T) {
	const base = "origin/main"
	clean := Facts{Branch: "feat", Ahead: 2, Unpushed: 0}

	with := func(mod func(*Facts)) Facts {
		f := clean
		mod(&f)
		return f
	}

	tests := []struct {
		name   string
		facts  Facts
		lookup PRLookup
		want   Result
	}{
		{"missing dir", Facts{Missing: true}, nil, Result{Prune, "directory gone"}},
		{"detached", with(func(f *Facts) { f.Branch = "" }), nil, Result{Review, "detached HEAD"}},
		{"dirty", with(func(f *Facts) { f.Dirty = 3 }), nil, Result{Review, "3 uncommitted change(s)"}},
		{"status failed", with(func(f *Facts) { f.Dirty = Unknown }), nil, Result{Review, "git status failed"}},
		{"dirty beats in-base", with(func(f *Facts) { f.Dirty = 1; f.InBase = true }), nil, Result{Review, "1 uncommitted change(s)"}},
		{"in base", with(func(f *Facts) { f.InBase = true }), nil, Result{Remove, "in origin/main"}},
		{"in base, no upstream", with(func(f *Facts) { f.InBase = true; f.Unpushed = Unknown }), nil, Result{Remove, "in origin/main"}},
		{"in base, unpushed", with(func(f *Facts) { f.InBase = true; f.Unpushed = 2 }), nil, Result{Review, "in origin/main but 2 unpushed commit(s)"}},
		{"pr merged", clean, prOf("MERGED", 7), Result{Remove, "PR #7 merged (squash/rebase)"}},
		{"pr closed", clean, prOf("CLOSED", 8), Result{Remove, "PR #8 closed unmerged"}},
		{"pr open", clean, prOf("OPEN", 9), Result{Keep, "PR #9 open"}},
		{"pr merged, unpushed", with(func(f *Facts) { f.Unpushed = 1 }), prOf("MERGED", 7), Result{Review, "PR #7 merged (squash/rebase) but 1 unpushed commit(s)"}},
		{"no pr", clean, func(string) *gh.PR { return nil }, Result{Review, "no PR, 2 commit(s) not in origin/main, unpushed=0"}},
		{"no gh", with(func(f *Facts) { f.Unpushed = Unknown }), nil, Result{Review, "no PR, 2 commit(s) not in origin/main, unpushed=?"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.facts, base, tt.lookup); got != tt.want {
				t.Errorf("Classify() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestClassifySkipsLookupWhenConclusive(t *testing.T) {
	called := false
	lookup := func(string) *gh.PR { called = true; return nil }
	Classify(Facts{Branch: "b", InBase: true}, "main", lookup)
	Classify(Facts{Branch: "b", Dirty: 1}, "main", lookup)
	if called {
		t.Error("PR lookup should not run when the ancestor/dirty checks decide")
	}
}
