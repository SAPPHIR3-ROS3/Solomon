set -euo pipefail

: "${GITHUB_REPOSITORY:?Repository required}"
: "${GITHUB_RUN_ID:?Workflow run required}"
base="repos/$GITHUB_REPOSITORY/actions"
ids=$(gh api --paginate "$base/runs/$GITHUB_RUN_ID/artifacts?per_page=100" --jq '.artifacts[] | select(.name | startswith("desktop-")) | .id')
failed=0
for id in $ids; do
  if ! output=$(gh api --include --method DELETE "$base/artifacts/$id" 2>&1); then
    if ! grep -Eq '^HTTP/[^ ]+ 404' <<< "$output"; then
      printf '%s\n' "$output" >&2
      failed=1
    fi
  fi
done
exit "$failed"
