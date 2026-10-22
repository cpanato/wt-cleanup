# wt-cleanup

Classify git worktrees and their branches, and optionally remove the ones whose
work has already landed.

It is safe by default. It prints a report and changes nothing until you pass `--apply`.

```
$ wt-cleanup
=== /src/myrepo ===
base=upstream/main  github=org/myrepo  fetched=2026-10-22 10:48
  myrepo-feat-a     feat-a        REMOVE  (PR #123 merged (squash/rebase))
  myrepo-feat-b     feat-b        KEEP    (PR #130 open)
  myrepo-spike      spike         REVIEW  (2 uncommitted change(s))

summary: 1 removable, 1 keep (open PR), 1 to review, 0 stale record(s)
dry run — re-run with --apply (add --yes to skip prompts) to act
```

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

## Install

Download a binary from [Releases](https://github.com/cpanato/wt-cleanup/releases), or:

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

## Development

```sh
go test ./...
golangci-lint run
```

To release, push a `v*` tag. GoReleaser builds binaries for darwin and linux on amd64 and arm64.
