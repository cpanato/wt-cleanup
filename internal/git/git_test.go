package git

import (
	"reflect"
	"testing"
)

func TestParseWorktrees(t *testing.T) {
	out := `worktree /src/repo
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /src/repo wt/with space
HEAD 2222222222222222222222222222222222222222
branch refs/heads/feat/x

worktree /src/detached
HEAD 3333333333333333333333333333333333333333
detached

worktree /src/gone
HEAD 4444444444444444444444444444444444444444
branch refs/heads/old
prunable gitdir file points to non-existent location
`
	want := []Worktree{
		{Path: "/src/repo", Branch: "main"},
		{Path: "/src/repo wt/with space", Branch: "feat/x"},
		{Path: "/src/detached", Detached: true},
		{Path: "/src/gone", Branch: "old"},
	}
	if got := ParseWorktrees(out); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseWorktrees() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestParseWorktreesBare(t *testing.T) {
	got := ParseWorktrees("worktree /src/repo.git\nbare\n")
	want := []Worktree{{Path: "/src/repo.git", Bare: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestNWOFromURL(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:cpanato/wt-cleanup.git":       "cpanato/wt-cleanup",
		"git@github.com:cpanato/wt-cleanup":           "cpanato/wt-cleanup",
		"https://github.com/cpanato/wt-cleanup.git":   "cpanato/wt-cleanup",
		"https://github.com/cpanato/wt-cleanup":       "cpanato/wt-cleanup",
		"ssh://git@github.com/cpanato/wt-cleanup.git": "cpanato/wt-cleanup",
		"http://ghe.example.com/org/repo.git":         "org/repo",
		"https://github.com/cpanato/wt-cleanup.git\n": "cpanato/wt-cleanup",
	} {
		if got := NWOFromURL(url); got != want {
			t.Errorf("NWOFromURL(%q) = %q, want %q", url, got, want)
		}
	}
}
