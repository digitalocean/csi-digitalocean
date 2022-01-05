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

set -e

if [[ $# -ne 1 ]]; then
  echo "usage: $(basename "$0") <Kubernetes semver version x.y.z>" >&2
  exit 1
fi

readonly KUBERNETES_VERSION="$1"

deps=()

while read -ra LINE
do
  depname="${LINE[0]}"
  deps+=("-replace $depname=$depname@kubernetes-$KUBERNETES_VERSION")
done < <(curl -fsSL "https://raw.githubusercontent.com/kubernetes/kubernetes/v$KUBERNETES_VERSION/go.mod" \
  | grep -E '^[[:space:]]*k8s.io.* v0.0.0$')

unset GOROOT GOPATH
export GO111MODULE=on

set -x
# shellcheck disable=SC2086
go mod edit ${deps[*]}
go mod tidy
go mod vendor
set +x
