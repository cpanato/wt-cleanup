// Package gh looks up pull request state through the gh CLI.
package gh

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"
	"time"
)

// PR is the newest pull request found for a head branch.
type PR struct {
	Number    int       `json:"number"`
	State     string    `json:"state"` // OPEN, MERGED or CLOSED
	UpdatedAt time.Time `json:"updatedAt"`
}

// Available reports whether the gh binary is on PATH.
func Available() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// LatestPR returns the most recently updated PR whose head is branch in the
// repo nwo ("owner/repo"). It returns nil when there is none or the lookup fails.
func LatestPR(ctx context.Context, nwo, branch string) *PR {
	out, err := exec.CommandContext(ctx, "gh", "pr", "list",
		"--repo", nwo, "--head", branch, "--state", "all", "--limit", "5",
		"--json", "number,state,updatedAt").Output()
	if err != nil {
		return nil
	}
	return newest(out)
}

func newest(data []byte) *PR {
	var prs []PR
	if err := json.Unmarshal(data, &prs); err != nil || len(prs) == 0 {
		return nil
	}
	sort.SliceStable(prs, func(i, j int) bool { return prs[i].UpdatedAt.After(prs[j].UpdatedAt) })
	return &prs[0]
}
