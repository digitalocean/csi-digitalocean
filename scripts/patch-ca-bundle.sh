#!/bin/bash
# File originally from https://github.com/banzaicloud/admission-webhook-example/blob/blog/deployment/webhook-patch-ca-bundle.sh

ROOT=$(cd $(dirname $0)/../../; pwd)

set -o errexit
set -o nounset
set -o pipefail

export CA_BUNDLE=$(kubectl config view --minify --raw -o json | jq -r '.clusters[0].cluster."certificate-authority-data"' | tr -d '"')

if command -v envsubst >/dev/null 2>&1; then
    envsubst
else
    sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g"
fi
