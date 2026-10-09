#!/usr/bin/env bash
set -euo pipefail

# GITHUB_TOKEN cannot publish a tag whose workflow files differ from the
# repository's default branch. A release token with Workflows: write can.
if [[ "${RELEASE_TOKEN_CONFIGURED:-false}" == "true" ]]; then
  exit 0
fi

git fetch --no-tags origin "${DEFAULT_BRANCH:?}"
if ! git diff --quiet FETCH_HEAD HEAD -- .github/workflows; then
  echo "::error::Release workflow files differ from ${DEFAULT_BRANCH}. Restart the Release workflow from the latest ${DEFAULT_BRANCH} commit, or configure the RELEASE_TOKEN repository secret with Contents: write and Workflows: write to publish this revision."
  exit 1
fi
