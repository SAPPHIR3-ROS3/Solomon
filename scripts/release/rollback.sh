set -euo pipefail

: "${RELEASE_TAG:?Release tag required}"
: "${PREVIOUS_TAG:?Previous stable tag required}"
: "${GITHUB_REPOSITORY:?Repository required}"
[[ "$RELEASE_TAG" != "$PREVIOUS_TAG" ]] || exit 1
base="repos/$GITHUB_REPOSITORY"

request() {
  local output attempt
  for attempt in 1 2 3; do
    if output=$(gh api --include "$@" 2>&1); then
      printf '%s\n' "$output"
      return 0
    fi
    if grep -Eq '^HTTP/[^ ]+ 404' <<< "$output"; then return 0; fi
    printf '%s\n' "$output" >&2
    sleep 1
  done
  return 1
}

response=$(request "$base/releases/tags/$RELEASE_TAG")
failed=0
if [[ -n "$response" ]]; then
  release_id=$(printf '%s\n' "$response" | tr -d '\r' | sed '1,/^$/d' | jq -er .id)
  request --method PATCH "$base/releases/$release_id" -F draft=true >/dev/null || failed=1
fi
gh release edit "$PREVIOUS_TAG" --repo "$GITHUB_REPOSITORY" --latest || failed=1
if [[ -n "${release_id:-}" ]]; then
  request --method DELETE "$base/releases/$release_id" >/dev/null || failed=1
fi
request --method DELETE "$base/git/refs/tags/$RELEASE_TAG" >/dev/null || failed=1
[[ "$failed" == 0 ]] || { echo 'Release rollback incomplete' >&2; exit 1; }
printf 'Withdrawn release: %s; restored latest: %s\n' "$RELEASE_TAG" "$PREVIOUS_TAG"
exit "$failed"
