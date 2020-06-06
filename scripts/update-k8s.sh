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

set -e

KUBERNETES_VERSION=${KUBERNETES_VERSION:?}

deps=()

while read -ra LINE
do
  depname="${LINE[0]}"
  deps+=("-replace $depname=$depname@kubernetes-$KUBERNETES_VERSION")
done < <(curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/v$KUBERNETES_VERSION/go.mod" \
  | grep -E '^[[:space:]]*k8s.io.* v0.0.0$')

deps+=("-replace k8s.io/kubernetes=k8s.io/kubernetes@v$KUBERNETES_VERSION")

unset GOROOT GOPATH
export GO111MODULE=on

set -x
# shellcheck disable=SC2086
go mod edit ${deps[*]}
go mod tidy
go mod vendor
set +x

sed -i -e "s/^KUBERNETES_VERSION.*/KUBERNETES_VERSION ?= $KUBERNETES_VERSION/" Makefile
rm Makefile-e
