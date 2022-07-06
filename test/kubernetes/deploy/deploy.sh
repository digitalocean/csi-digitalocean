#!/bin/bash

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

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
readonly REPO_SCRIPTS_DIR="${SCRIPT_DIR}/../../../scripts"
readonly DEV_RELEASE_DIR="${SCRIPT_DIR}/../../../deploy/kubernetes/releases/csi-digitalocean-dev"
readonly DEFAULT_PLUGIN_IMAGE='digitalocean/do-csi-plugin:dev'

YES=
if [[ $# -gt 0 && ( $1 = '-y' || $1 = '--yes' ) ]]; then
    YES=1
fi
readonly YES

if ! command -v kustomize >/dev/null 2>&1; then
    echo 'kustomize not installed'
    echo 'get it from https://github.com/kubernetes-sigs/kustomize'
    exit 1
fi
if ! command -v kubectl >/dev/null 2>&1; then
    echo 'kubectl not installed'
    echo 'install it following the instructions at https://kubernetes.io/docs/tasks/tools/install-kubectl/'
    exit 1
fi

if [[ -z "${DIGITALOCEAN_ACCESS_TOKEN:-}" ]]; then
    echo 'DIGITALOCEAN_ACCESS_TOKEN not defined'
    exit 1
fi

current_context=$(kubectl config current-context)
echo "Deploying a dev version of the CSI driver to context ${current_context}."
if [[ -z "${YES}" ]]; then
    echo "Continue? (yes/no)"
    read -r yesno
    if [[ "${yesno}" != 'yes' ]]; then
        echo 'Aborted'
        exit 1
    fi
fi

# Create a secret containing the specified DO API token; this will be used by
# the dev version of the CSI controller.
# Piping the dry-run YAML output to kubectl apply is a common trick to implement
# upsert semantics with secrets specified imperatively.
kubectl -n kube-system create secret generic digitalocean --from-literal="access-token=${DIGITALOCEAN_ACCESS_TOKEN}" --dry-run=client -o yaml |
    kubectl apply -f -

"${REPO_SCRIPTS_DIR}/create-cert.sh" --service snapshot-validation-service-dev --secret snapshot-validation-secret --namespace kube-system

# Configure kustomize to use the specified dev image (default to the one created
# by `VERSION=dev make publish`).
: "${DEV_IMAGE:=$DEFAULT_PLUGIN_IMAGE}"
(
cd "${SCRIPT_DIR}"
kustomize edit set image digitalocean/do-csi-plugin="${DEV_IMAGE}"
# Undo any image updates done to kustomization.yaml to prevent git pollution.
# shellcheck disable=SC2064
trap "kustomize edit set image digitalocean/do-csi-plugin=$DEFAULT_PLUGIN_IMAGE" EXIT

# Apply the CRDs.
kubectl apply -f "${DEV_RELEASE_DIR}/crds.yaml"

echo -n 'Waiting for CRDs to become established'
while [[ $(kubectl get crd volumesnapshotclasses.snapshot.storage.k8s.io -o jsonpath='{.status.conditions[?(@.type == "Established")].status}') != "True" ]]; do
    echo -n '.'
    sleep 2
done
echo

# Apply the customization to the dev manifests, and install them into the cluster.
kustomize build . --load-restrictor LoadRestrictionsNone | "${REPO_SCRIPTS_DIR}/patch-ca-bundle.sh" | kubectl apply -f -
)

# Wait for the deployment to complete.
kubectl -n kube-system wait --timeout=5m --for=condition=Ready pod -l app=csi-do-controller-dev
kubectl -n kube-system wait --timeout=5m --for=condition=Ready pod -l app=csi-do-node-dev
kubectl -n kube-system wait --timeout=5m --for=condition=Ready pod -l app=snapshot-controller
kubectl -n kube-system wait --timeout=5m --for=condition=Ready pod -l app=snapshot-validation
kubectl -n kube-system get all
