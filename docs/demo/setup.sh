#!/usr/bin/env bash
# Builds a throwaway repo with one worktree per verdict for the README demo.
# Usage: docs/demo/setup.sh <dir>
set -euo pipefail

dir=${1:?usage: setup.sh <dir>}
rm -rf "$dir" && mkdir -p "$dir"
cd "$dir"

export GIT_CONFIG_GLOBAL="$dir/gitconfig" GIT_CONFIG_NOSYSTEM=1
git config --global user.name demo
git config --global user.email demo@example.com
git config --global init.defaultBranch main
g() { git -C "$1" "${@:2}" >/dev/null 2>&1; }

git init -q --bare origin.git
git clone -q origin.git myrepo 2>/dev/null
g myrepo commit --allow-empty -m init
g myrepo push -u origin main

wt() { g myrepo worktree add -b "$1" "$dir/myrepo-$1" main; }

for b in fix-typo add-flag; do
  wt "$b"
  g "myrepo-$b" commit --allow-empty -m "$b"
  g myrepo merge --ff-only "$b"
done
g myrepo push origin main

wt spike
g myrepo-spike commit --allow-empty -m spike

wt refactor
echo wip > myrepo-refactor/notes.txt

wt old-experiment
rm -rf myrepo-old-experiment

# Show a GitHub-style remote in the header; the demo runs with --no-gh.
g myrepo remote set-url origin git@github.com:example/myrepo.git
