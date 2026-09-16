#!/usr/bin/env bash

# Copyright 2026 DigitalOcean
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

# Dumps CSI-related cluster state after a failed end-to-end run. The upstream
# test framework only collects events from the test namespaces, which leaves no
# trace of what the CSI sidecars did, so attach/detach and volume deletion
# failures cannot be diagnosed from the job log alone.
#
# Every command is best-effort: a failure here must not mask the test failure.

set -u

readonly TAIL_LINES="${DUMP_TAIL_LINES:-2000}"
readonly CONTROLLER_SELECTOR='app=csi-do-controller-dev'

section() {
    echo
    echo "===== $* ====="
}

section 'kube-system pods'
kubectl -n kube-system get pods -o wide

section 'VolumeAttachments'
kubectl get volumeattachments -o wide

# Attachments lingering past the test are the ones that block volume deletion,
# so keep their full spec including finalizers and deletion timestamps.
section 'VolumeAttachments (full)'
kubectl get volumeattachments -o yaml

section 'PersistentVolumes'
kubectl get pv -o wide

section 'PersistentVolumes (full)'
kubectl get pv -o yaml

for container in csi-attacher csi-provisioner csi-resizer csi-do-plugin; do
    section "csi-do-controller-dev logs: ${container}"
    kubectl -n kube-system logs \
        --selector="${CONTROLLER_SELECTOR}" \
        --container="${container}" \
        --tail="${TAIL_LINES}"
done

exit 0
