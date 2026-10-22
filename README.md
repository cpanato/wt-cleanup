# wt-cleanup

[![ci](https://github.com/cpanato/wt-cleanup/actions/workflows/ci.yaml/badge.svg)](https://github.com/cpanato/wt-cleanup/actions/workflows/ci.yaml)
[![release](https://img.shields.io/github/v/release/cpanato/wt-cleanup)](https://github.com/cpanato/wt-cleanup/releases/latest)

Classify git worktrees and their branches, and optionally remove the ones whose
work has already landed.

It is safe by default. It prints a report and changes nothing until you pass `--apply`.

![wt-cleanup demo](docs/demo/demo.gif)

## How it classifies

The primary worktree is never touched. Every other worktree gets one verdict:

| Verdict  | When | With `--apply` |
|----------|------|----------------|
| `PRUNE`  | The directory is gone and only git's admin record remains. | `git worktree prune` |
| `REMOVE` | The branch is an ancestor of the base, or its PR is merged or closed, and the tree is clean with no unpushed commits. | Remove the worktree and delete the branch |
| `KEEP`   | The branch has an open PR. | Nothing |
| `REVIEW` | Anything else: no PR and not merged, a dirty tree, unpushed commits, or a detached HEAD. | Nothing |

The base is `--base` if you set it. Otherwise it is the first of these that exists: `upstream/main`, `upstream/master`, `origin/main`, `origin/master`, `main`, `master`.

PR state comes from the [`gh`](https://cli.github.com/) CLI, using the `upstream` remote or, failing that, `origin`. If `gh` is not installed, the tool behaves as if you passed `--no-gh`.

## What it never touches

- The primary worktree.
- Stashes. They are shared by all worktrees, so the tool counts them in the header and leaves them alone.
- Worktrees with uncommitted changes or a detached HEAD. These are always REVIEW.
- Branches with commits that their upstream doesn't have. These are REVIEW even when the PR is merged.
- Anything at all, unless you pass `--apply`. With `--apply` it asks before each removal unless you add `--yes`.

Worktrees are removed with plain `git worktree remove`, without `--force`, so git itself refuses locked or modified trees.

One caveat: the unpushed check needs a live upstream branch. If the remote branch was deleted (common after a merge), a branch whose PR is merged or closed is still REMOVE, and any local commits made after the push are lost with it. Check the REMOVE list before you apply if that matters to you.

## Install

With Homebrew (macOS and Linux):

```sh
brew install cpanato/tap/wt-cleanup
```

Or download a binary from [Releases](https://github.com/cpanato/wt-cleanup/releases), or use Go:

```sh
go install github.com/cpanato/wt-cleanup@latest
```

`git` must be on your `PATH`. `gh` is optional.

## Usage

```
wt-cleanup [flags] [repo-path ...]
```

With no paths, it uses the repo containing the current directory. You can run it from any worktree.

| Flag | Description |
|------|-------------|
| `--apply` | Remove worktrees and delete branches. Without it, the tool only reports. |
| `-y`, `--yes` | With `--apply`, skip the prompt for each worktree. |
| `--fetch` | Run `git fetch --all --prune` first. Over SSH, this may ask for a security key touch. |
| `--base <ref>` | The ref to test against for "already landed". |
| `--no-gh` | Skip GitHub PR lookups and use only the ancestor test. Works offline. |
| `--caches` | Also report `.terraform` and `node_modules` directories in the surviving worktrees. With `--apply`, delete them. |

Run `wt-cleanup version` to print build info, or `wt-cleanup version --json` for JSON.

## Development

```sh
go test ./...
golangci-lint run
```

To regenerate the demo GIF, run `vhs docs/demo/demo.tape` from the repo root. This needs [vhs](https://github.com/charmbracelet/vhs).

To release, push a `v*` tag. GoReleaser builds binaries for darwin and linux on amd64 and arm64.
