#!/bin/bash

set -euo pipefail

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
echo "Continue? (yes/no)"
read -r yesno
if [[ "${yesno}" != 'yes' ]]; then
    echo 'Aborted'
    exit 1
fi

# Create a secret containing the specified DO API token; this will be used by
# the dev version of the CSI controller.
# Piping the dry-run YAML output to kubectl apply is a common trick to implement
# upsert semantics with secrets specified imperatively.
kubectl -n kube-system create secret generic digitalocean --from-literal="access-token=${DIGITALOCEAN_ACCESS_TOKEN}" --dry-run -o yaml |
    kubectl apply -f -

# Configure kustomize to use the specified dev image (default to the one created
# by `VERSION=dev make publish`).
: "${DEV_IMAGE:=digitalocean/do-csi-plugin:dev}"
kustomize edit set image digitalocean/do-csi-plugin:dev="${DEV_IMAGE}"
# Apply the customization to the dev manifest, and apply it to the cluster.
kustomize build . --load_restrictor none | kubectl apply -f -
# Wait for the deployment to complete.
kubectl -n kube-system wait --timeout=5m --for=condition=Ready pod -l app=csi-do-controller-dev
kubectl -n kube-system wait --timeout=5m --for=condition=Ready pod -l app=csi-do-node-dev
kubectl -n kube-system get all
