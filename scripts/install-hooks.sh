#!/usr/bin/env bash
# Install repository git hooks into .git/hooks (idempotent).
#
# Without this, the hooks in scripts/git-hooks/ are inert because git only
# executes hooks from .git/hooks/. Re-run safely; existing symlinks are
# replaced, but a real file at the destination is preserved (with a warning).
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
hooks_src="${repo_root}/scripts/git-hooks"
hooks_dst="$(git rev-parse --git-path hooks)"

mkdir -p "$hooks_dst"

for src in "$hooks_src"/*; do
  name="$(basename "$src")"
  dst="${hooks_dst}/${name}"
  if [[ -e "$dst" && ! -L "$dst" ]]; then
    echo "warning: ${dst} exists and is not a symlink; leaving it alone." >&2
    continue
  fi
  ln -sfn "$src" "$dst"
  chmod +x "$src"
  echo "installed: ${name}"
done

echo "done. Hooks installed under: ${hooks_dst}"
