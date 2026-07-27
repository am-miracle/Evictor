#!/usr/bin/env bash

set -euo pipefail

test_log="$(mktemp)"
trap 'rm -f "$test_log"' EXIT

(
  cd backend
  go test ./...
) 2>&1 | tee "$test_log"

if grep -E \
  'api-key-material|provider-material|credential-material|encryption-secret|evictor_test_api_key_do_not_log' \
  "$test_log"; then
  echo "credential material found in backend test logs" >&2
  exit 1
fi
