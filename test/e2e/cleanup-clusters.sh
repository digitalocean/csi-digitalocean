#!/usr/bin/env bash

# Copyright 2022 DigitalOcean
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

### Delete clusters that match a given DO tag along with all managed volumes that
### may still exist.

set -o errexit
set -o nounset

if ! type doctl > /dev/null 2>&1; then
  echo "doctl is missing" >&2
  exit 1
fi

if [[ $# -ne 1 ]]; then
  echo "usage: $(basename "$0") <identifying cluster tag>" >&2
  exit 1
fi

readonly CLUSTER_TAG="$1"

echo "Cleaning up clusters based on cluster tag: ${CLUSTER_TAG}"

while read -r cluster_uuid; do
  if doctl kubernetes cluster get "${cluster_uuid}" --no-header --format Tags | grep -q "${CLUSTER_TAG}"; then
    echo "Deleting cluster ${cluster_uuid}"
    doctl kubernetes cluster delete -f "${cluster_uuid}"
    echo "Deleting volumes for cluster ${cluster_uuid}"
    doctl compute volume list --no-header --format ID,Tags | grep "k8s:${cluster_uuid}" | awk '{print $1}' | xargs -n 1 doctl compute volume delete -f
  fi
done < <(doctl kubernetes cluster list --no-header --format ID)   # --format field "Tags" does not work on the "list" command
echo "Done."
