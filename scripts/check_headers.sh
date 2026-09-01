#!/usr/bin/env bash
set -euo pipefail
fail=0
while IFS= read -r f; do
  if ! head -n1 "$f" | grep -q 'SPDX-License-Identifier: Apache-2.0'; then
    echo "missing SPDX header: $f"
    fail=1
  fi
done < <(find . -name '*.go' -not -path './vendor/*')
exit "$fail"
