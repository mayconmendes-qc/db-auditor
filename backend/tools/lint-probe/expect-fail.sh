#!/bin/sh
# Proves a new gosec finding fails the linter. The fixture is outside the main packages.
set -eu
root="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
if command -v golangci-lint >/dev/null 2>&1; then
  lint=golangci-lint
else
  echo "golangci-lint is not installed; skipping local probe"
  exit 0
fi
# Own config: the backend config excludes this fixture so the main lint stays green.
if (cd "$root" && "$lint" run --config .golangci.yml .) >/tmp/gosec-probe.out 2>&1; then
  echo "gosec accepted a known weak hash"
  cat /tmp/gosec-probe.out
  exit 1
fi
echo "gosec rejected the fixture"
