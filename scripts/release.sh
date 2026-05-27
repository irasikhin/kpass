#!/usr/bin/env bash
# Cut a new release.
#
# Usage:
#   scripts/release.sh <version>      # explicit semver, e.g. 0.3.0
#   scripts/release.sh patch|minor|major   # auto-bump from the last tag
#
# What it does:
#   1. Refuses to run on a dirty working tree.
#   2. Computes the next version from your argument.
#   3. Generates a CHANGELOG.md section from `git log <prev-tag>..HEAD`
#      grouped by Conventional Commit type. The user is dropped into $EDITOR
#      to review/edit before committing.
#   4. Updates `version` in nix/package.nix to match the new tag.
#   5. Commits with `chore(release): vX.Y.Z` and tags `vX.Y.Z`.
#
# The GitHub release workflow takes it from there:
#   git push --follow-tags
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <patch|minor|major|X.Y.Z>" >&2
  exit 2
fi
bump="$1"

if ! git diff-index --quiet HEAD --; then
  echo "error: working tree is dirty; commit or stash first." >&2
  exit 1
fi

# Last v* tag (lexicographic is wrong; use --sort).
prev_tag="$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1 || true)"
if [[ -z "$prev_tag" ]]; then
  prev_tag="v0.0.0"
fi
prev_version="${prev_tag#v}"

case "$bump" in
  patch|minor|major)
    IFS='.' read -r major minor patch <<<"$prev_version"
    case "$bump" in
      major) major=$((major+1)); minor=0; patch=0 ;;
      minor) minor=$((minor+1)); patch=0 ;;
      patch) patch=$((patch+1)) ;;
    esac
    new_version="${major}.${minor}.${patch}"
    ;;
  *)
    if ! [[ "$bump" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$ ]]; then
      echo "error: '$bump' is not a valid semver (X.Y.Z) or bump keyword." >&2
      exit 1
    fi
    new_version="$bump"
    ;;
esac
new_tag="v${new_version}"

if git rev-parse "$new_tag" >/dev/null 2>&1; then
  echo "error: tag ${new_tag} already exists." >&2
  exit 1
fi

echo "Releasing ${prev_tag} -> ${new_tag}"

# --- Build the changelog section from conventional commits ------------------

scratch="$(mktemp)"
trap 'rm -f "$scratch"' EXIT

today="$(date -u +%Y-%m-%d)"
{
  printf '## [%s] — %s\n\n' "$new_version" "$today"

  # Group commit subjects by type. Skip merge / chore(release) / non-conv lines.
  declare -A buckets=(
    [feat]="Added" [fix]="Fixed" [perf]="Performance"
    [refactor]="Refactored" [docs]="Docs" [test]="Tests"
    [build]="Build" [ci]="CI" [chore]="Chores" [revert]="Reverts" [style]="Style"
  )
  # Order in the changelog section.
  order=(feat fix perf refactor docs test build ci chore revert style)

  declare -A out
  while IFS=$'\t' read -r sha subject; do
    [[ -z "$subject" ]] && continue
    [[ "$subject" == Merge* ]] && continue
    [[ "$subject" == chore\(release\):* ]] && continue
    if [[ "$subject" =~ ^([a-z]+)(\([^\)]+\))?!?:\ (.+) ]]; then
      type="${BASH_REMATCH[1]}"
      desc="${BASH_REMATCH[3]}"
      out[$type]+="- ${desc} (${sha})"$'\n'
    fi
  done < <(git log --reverse --no-merges "${prev_tag}..HEAD" --format='%h%x09%s')

  any=0
  for t in "${order[@]}"; do
    if [[ -n "${out[$t]:-}" ]]; then
      printf '### %s\n\n%s\n' "${buckets[$t]}" "${out[$t]}"
      any=1
    fi
  done
  if [[ "$any" -eq 0 ]]; then
    printf '_No conventional-commit entries since %s._\n\n' "$prev_tag"
  fi
} > "$scratch"

# --- Splice the new section into CHANGELOG.md -------------------------------

if [[ ! -f CHANGELOG.md ]]; then
  echo "error: CHANGELOG.md not found at repo root." >&2
  exit 1
fi

# Insert the new section right after the "## [Unreleased]" block, OR before
# the first existing "## [" block if no Unreleased section is present.
python3 - "$scratch" CHANGELOG.md "$new_version" "$prev_version" <<'PY'
import re, sys, pathlib
section, changelog, new_v, prev_v = sys.argv[1:5]
text = pathlib.Path(changelog).read_text()
new_block = pathlib.Path(section).read_text()

# Find the start of the next "## [" after the Unreleased block, or fall back
# to the first "## [" header.
m = re.search(r'^## \[Unreleased\][^\n]*\n', text, re.M)
insert_at = None
if m:
    # Locate the next "## [" after Unreleased.
    rest = re.search(r'^## \[', text[m.end():], re.M)
    insert_at = m.end() + (rest.start() if rest else len(text) - m.end())
else:
    m2 = re.search(r'^## \[', text, re.M)
    insert_at = m2.start() if m2 else len(text)

text = text[:insert_at] + new_block + "\n" + text[insert_at:]

# Refresh the link refs at the bottom (best-effort: add a new compare link).
ref_block = f"[{new_v}]: https://github.com/irasikhin/kpass/compare/v{prev_v}...v{new_v}\n"
if ref_block.strip() not in text:
    text = text.rstrip() + "\n" + ref_block

pathlib.Path(changelog).write_text(text)
PY

# --- Let the user review the inserted block ---------------------------------

# Review the changelog if running interactively (stdin is a tty), otherwise
# accept the generated section as-is.
if [[ -t 0 ]]; then
  ${EDITOR:-vi} CHANGELOG.md
fi

# --- Update nix/package.nix version ----------------------------------------

if [[ ! -f nix/package.nix ]]; then
  echo "error: nix/package.nix not found." >&2
  exit 1
fi
sed -i -E "s/^(\s*version\s*=\s*\")[^\"]+(\";)/\1${new_version}\2/" nix/package.nix

if ! grep -qE "version\s*=\s*\"${new_version}\";" nix/package.nix; then
  echo "error: failed to update version in nix/package.nix." >&2
  exit 1
fi

# --- Commit + tag -----------------------------------------------------------

git add CHANGELOG.md nix/package.nix
git commit -m "chore(release): ${new_tag}"
git tag -a "${new_tag}" -m "${new_tag}"

cat <<EOF

Release prepared: ${new_tag}

Next: review the commit, then publish with
  git push --follow-tags
EOF
