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

### base build containers
FROM golang:1.13 AS builder

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends rsync
RUN mkdir -p /go/src/k8s.io
WORKDIR /go/src/k8s.io

### Kubernetes 1.26
FROM builder AS tests-1.26
ARG KUBE_VERSION_1_26=1.26.0
ARG KUBE_VERSION_1_26_E2E_BIN_SHA256_CHECKSUM=6d337b8bd7789105e95682ff1ad7da8d3e8c6c40e8d85021ef72d4ae117f08a9

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_26}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_26_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.26.test
RUN cp ginkgo /ginkgo-1.26

### Kubernetes 1.25
FROM builder AS tests-1.25
ARG KUBE_VERSION_1_25=1.25.2
ARG KUBE_VERSION_1_25_E2E_BIN_SHA256_CHECKSUM=b9ec7494abed4d5d40f4027e4211da0a2b33f76f81f6813430a9ebd142ca83a0

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_25}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_25_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.25.test
RUN cp ginkgo /ginkgo-1.25

### Kubernetes 1.24
FROM builder AS tests-1.24
ARG KUBE_VERSION_1_24=1.24.3
ARG KUBE_VERSION_1_24_E2E_BIN_SHA256_CHECKSUM=94139dbaaf944c762be269bfe6ed389029722d8d19b88e6d10c6730dbf6a98b2

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_24}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_24_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.24.test
RUN cp ginkgo /ginkgo-1.24

### Kubernetes 1.23
FROM builder AS tests-1.23
ARG KUBE_VERSION_1_23=1.23.7
ARG KUBE_VERSION_1_23_E2E_BIN_SHA256_CHECKSUM=e1a9a632b7715d4bfbac3322da5d32072eaf544b31a81a6c3df205d0dc0e96b7

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_23}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_23_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.23.test
RUN cp ginkgo /ginkgo-1.23

FROM golang:1.19 AS tools
# See comment at the bottom on why we need tini.
ARG TINI_VERSION=0.18.0
# doctl is needed to support clusters that had their kubeconfig fetched via the
# CLI because those leverage a kubeconfig authentication plugin based on doctl.
ARG DOCTL_VERSION=1.76.2

RUN curl --fail --location --output /tini https://github.com/krallin/tini/releases/download/v${TINI_VERSION}/tini
RUN chmod u+x /tini

RUN curl --fail --location https://github.com/digitalocean/doctl/releases/download/v${DOCTL_VERSION}/doctl-${DOCTL_VERSION}-linux-amd64.tar.gz | tar -xzv
RUN cp doctl /

RUN curl --fail --location --remote-name https://storage.googleapis.com/kubernetes-release/release/$(curl --fail --silent https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod u+x kubectl
RUN cp kubectl /

### final test container
FROM bitnami/minideb:buster AS runtime
# Certificates needed to trust the CA for any HTTPS connections to the DO API.
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends ca-certificates
COPY --from=tests-1.26 /e2e.1.26.test /
COPY --from=tests-1.26 /ginkgo-1.26 /usr/local/bin
COPY --from=tests-1.25 /e2e.1.25.test /
COPY --from=tests-1.25 /ginkgo-1.25 /usr/local/bin
COPY --from=tests-1.24 /e2e.1.24.test /
COPY --from=tests-1.24 /ginkgo-1.24 /usr/local/bin
COPY --from=tests-1.23 /e2e.1.23.test /
COPY --from=tests-1.23 /ginkgo-1.23 /usr/local/bin
COPY --from=tools /tini /sbin/
COPY --from=tools /doctl /usr/local/bin/
COPY --from=tools /kubectl /usr/local/bin/

COPY cleanup-clusters.sh /

# Docker comes with built-in tini support (--init parameter) but does not allow
# to enable child process group killing
# (https://github.com/krallin/tini#process-group-killing) via "-g". We need this
# since our entrypoint script spawns child processes during multiple invocations
# of ginkgo. The usual approach of using "exec" to replace the shell does not
# work here as that would terminate the script prematurely.
# We also enable subreaping (https://github.com/krallin/tini#subreaping) to fix
# a startup warning.
# See also https://hynek.me/articles/docker-signals/ for the usual pid 1
# gotchas.
ENTRYPOINT ["/sbin/tini", "-g", "-s", "--", "/run-versioned-e2e-tests.sh"]
COPY run-versioned-e2e-tests.sh /
