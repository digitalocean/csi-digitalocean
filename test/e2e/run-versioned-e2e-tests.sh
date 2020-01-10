#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $(basename "$0") <Kubernetes major/minor version> <testdriver file>

<Kubernetes major/minor version>: Kubernetes major/minor version specifier to test.
<testdriver file>: testdriver file compatible with the given Kubernetes version to use.

Example:

$(basename "$0") 1.16 testdriver.1.16.yaml" >&2
  exit 1
fi

readonly KUBE_VER="$1"
readonly TD_FILE="$2"

E2E_TEST_FILE="/e2e.${KUBE_VER}.test"
if [[ ! -f "${E2E_TEST_FILE}" ]]; then
  echo "no e2e.test binary for Kubernetes version ${E2E_TEST_FILE} available" >&2
  exit 1
fi

if [[ ! -f "${TD_FILE}" ]]; then
  echo "testdriver file ${TD_FILE} does not exist" >&2
  exit 1
fi

focus=
if [[ "${FOCUS:-}" ]]; then
  focus=".*${focus}"
fi

if [[ "${SKIP_PARALLEL_TESTS:-}" ]]; then
  echo 'Skipping parallel tests'
else
  echo 'Running parallel tests'
  # Set node count explicitly since node detection does not work properly inside a
  # container.
  ginkgo -v -p -nodes 10 -focus="External.Storage${focus}" -skip='\[Feature:|\[Disruptive\]|\[Serial\]' "${E2E_TEST_FILE}" -- "-storage.testdriver=${TD_FILE}"
fi

if [[ "${SKIP_SEQUENTIAL_TESTS:-}" ]]; then
  echo 'Skipping sequential tests'
else
  echo 'Running sequential tests'
  ginkgo -v -focus="External.Storage${focus}.*(\[Feature:|\[Serial\])" -skip='\[Disruptive\]' "${E2E_TEST_FILE}" -- "-storage.testdriver=${TD_FILE}"
fi
