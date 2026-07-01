#!/usr/bin/env bash
set -euo pipefail

if [[ "${FORCE_TESTS:-false}" == "true" ]]; then
  echo "force_tests enabled; will run test suite"
  echo "skip_tests=false" >> "${GITHUB_OUTPUT}"
  exit 0
fi

sha="${GITHUB_SHA:?}"
repo="${GITHUB_REPOSITORY:?}"
matrix_os=(ubuntu-latest windows-latest macos-latest)

mapfile -t successful_checks < <(
  gh api "/repos/${repo}/commits/${sha}/check-runs?per_page=100" --paginate \
    --jq '.check_runs[] | select(.app.slug == "github-actions") | select(.conclusion == "success") | .name'
)

for os in "${matrix_os[@]}"; do
  pattern="test (${os})"
  found=false
  for name in "${successful_checks[@]}"; do
    if [[ "$name" == *"${pattern}"* ]]; then
      found=true
      break
    fi
  done
  if [[ "$found" != true ]]; then
    echo "No successful prior CI run for ${pattern} on ${sha}"
    echo "skip_tests=false" >> "${GITHUB_OUTPUT}"
    exit 0
  fi
done

echo "Test suite already passed on ${sha}; skipping duplicate test run"
echo "skip_tests=true" >> "${GITHUB_OUTPUT}"
