set -e
notes=$(find changelog -name "${VER}.md" -print -quit || true)
if [ -z "$notes" ]; then
  notes=$(mktemp)
  prev=$(git describe --tags --abbrev=0 --match 'v*' 'HEAD^' 2>/dev/null || true)
  range=HEAD
  if [ -n "$prev" ]; then range="${prev}..HEAD"; fi
  {
    echo "# ${VER}"
    echo
    echo "Calendar release ${VER}."
    echo
    for heading in Added Fixed Changed; do
      case "$heading" in
        Added) filter='^feat' ;;
        Fixed) filter='^fix' ;;
        Changed) filter='^(docs|chore|ci|build|test|refactor|perf)' ;;
      esac
      items=$(git log --format='%s' "$range" | grep -Ei "$filter" | grep -Eiv '^(Merge |build: include embedded assets)' || true)
      if [ -n "$items" ]; then
        echo "## ${heading}"
        echo
        echo "$items" | sed -E 's/^[A-Za-z]+(\([^)]+\))?!?: */- /'
        echo
      fi
    done
  } > "$notes"
fi
gh release create "$VER" dist/* \
  --title "$VER" \
  --notes-file "$notes"
