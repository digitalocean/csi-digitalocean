#!/usr/bin/env bash

# Copyright 2020 DigitalOcean
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
