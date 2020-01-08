#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

readonly IMAGE=${IMAGE:-digitalocean/k8s-e2e-test-runner}
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

usage() {
  echo "$(basename "$0") build | push"
}

if [[ $# -ne 1 ]]; then
  usage >&2
  exit 1
fi

readonly OPERATION="$1"

case "${OPERATION}" in
  build)
    (
    cd "${SCRIPT_DIR}"
    docker build -t "${IMAGE}" --build-arg SHA_1_16 --build-arg SHA_1_15 --build-arg SHA_1_14 .
    )
    ;;
  
  push)
    docker push "${IMAGE}"
    ;;

  *)
    usage >&2
    exit 1
esac
