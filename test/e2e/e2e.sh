#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

TIMEOUT="${TIMEOUT:-30m}"

(
  cd "${SCRIPT_DIR}"
  go test -v -tags e2e -test.timeout "${TIMEOUT}" ./... -args -long "$@"
)
