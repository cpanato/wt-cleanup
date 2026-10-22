// Command wt-cleanup classifies git worktrees and their branches and
// (optionally) removes the ones whose work has already landed.
package main

import (
	"context"
	"fmt"
	"os"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/cpanato/wt-cleanup/internal/cleanup"
	"github.com/cpanato/wt-cleanup/internal/gh"
)

// Set by goreleaser.
var (
	version = ""
	commit  = ""
)

const long = `Classify git worktrees + their branches and (optionally) remove the ones
whose work has already landed.

Safe by default: prints a report and changes nothing. Add --apply to act.

Classification per worktree (the primary worktree is never touched):
  PRUNE    directory is gone, only the admin record remains -> git worktree prune
  REMOVE   branch is an ancestor of <base>, or its PR is MERGED/CLOSED, and the
           tree is clean with nothing unpushed -> remove worktree + delete branch
  KEEP     branch has an OPEN PR
  REVIEW   no PR and not merged, or dirty tree, or unpushed commits -> left alone

With no repo-path arguments, the repo containing the current directory is used.`

func newRootCmd() *cobra.Command {
	var opts cleanup.Options
	noGH := false

	cmd := &cobra.Command{
		Use:          "wt-cleanup [flags] [repo-path ...]",
		Short:        "Clean up git worktrees whose work has landed",
		Long:         long,
		Args:         cobra.ArbitraryArgs, // repo paths, not subcommands
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				args = []string{wd}
			}
			opts.UseGH = !noGH
			if opts.UseGH && !gh.Available() {
				fmt.Fprintln(os.Stderr, "note: gh not found, falling back to --no-gh")
				opts.UseGH = false
			}

			r := cleanup.New(opts)
			for _, repo := range args {
				// Problems with one repo are reported inline; keep going.
				_ = r.Repo(cmd.Context(), repo)
			}
			r.Summary()
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&opts.Apply, "apply", false, "actually remove worktrees / delete branches (default: dry run)")
	f.BoolVarP(&opts.Yes, "yes", "y", false, "with --apply, don't prompt per worktree")
	f.BoolVar(&opts.Fetch, "fetch", false, "git fetch --prune every remote first (SSH: expect a YubiKey touch)")
	f.StringVar(&opts.Base, "base", "", "merge base to test against (default: upstream/main, else origin/main, else main)")
	f.BoolVar(&noGH, "no-gh", false, "skip GitHub PR lookups (offline; ancestor test only)")
	f.BoolVar(&opts.Caches, "caches", false, "also report (and with --apply delete) .terraform / node_modules caches in the surviving trees")
	return cmd
}

func main() {
	if err := fang.Execute(context.Background(), newRootCmd(),
		fang.WithVersion(version), fang.WithCommit(commit)); err != nil {
		os.Exit(1)
	}
}
