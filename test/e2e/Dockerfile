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
FROM golang:1.23 AS builder

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends rsync
RUN mkdir -p /go/src/k8s.io
WORKDIR /go/src/k8s.io

### Kubernetes 1.34
FROM builder AS tests-1.34
ARG KUBE_VERSION_1_34=1.34.1
ARG KUBE_VERSION_1_34_E2E_BIN_SHA256_CHECKSUM=4e2a8709a91f0476d7bfc3279867fe00380949ef6254066dcd8479f63b961277

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_34}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_34_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.34.test
RUN cp ginkgo /ginkgo-1.34

### Kubernetes 1.33
FROM builder As tests-1.33
ARG KUBE_VERSION_1_33=1.33.0
ARG KUBE_VERSION_1_33_E2E_BIN_SHA256_CHECKSUM=3c44921a69c56cda7a8c4a6f8ac081a8c5fba8742922c0707c9723ce8c4fbdb8

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_33}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_33_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.33.test
RUN cp ginkgo /ginkgo-1.33

### Kubernetes 1.32
FROM builder As tests-1.32
ARG KUBE_VERSION_1_32=1.32.3
ARG KUBE_VERSION_1_32_E2E_BIN_SHA256_CHECKSUM=c6b9a9a0fe096400a68f6309c46e43edbd0164afc4bc7f0608fa7eb401d6e76d

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_32}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_32_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.32.test
RUN cp ginkgo /ginkgo-1.32

### Kubernetes 1.31
FROM builder As tests-1.31
ARG KUBE_VERSION_1_31=1.31.7
ARG KUBE_VERSION_1_31_E2E_BIN_SHA256_CHECKSUM=8c10ed6ba538cb9fe181b1150e14813e55a73a2db0044c6f014a2e41d98f34ec

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_31}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_31_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.31.test
RUN cp ginkgo /ginkgo-1.31

FROM golang:1.23 AS tools
# See comment at the bottom on why we need tini.
ARG TINI_VERSION=0.19.0
# doctl is needed to support clusters that had their kubeconfig fetched via the
# CLI because those leverage a kubeconfig authentication plugin based on doctl.
ARG DOCTL_VERSION=1.146.0

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
    DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends ca-certificates \
COPY --from=tests-1.34 /e2e.1.34.test /
COPY --from=tests-1.34 /ginkgo-1.34 /usr/local/bin
COPY --from=tests-1.33 /e2e.1.33.test /
COPY --from=tests-1.33 /ginkgo-1.33 /usr/local/bin
COPY --from=tests-1.32 /e2e.1.32.test /
COPY --from=tests-1.32 /ginkgo-1.32 /usr/local/bin
COPY --from=tests-1.31 /e2e.1.31.test /
COPY --from=tests-1.31 /ginkgo-1.31 /usr/local/bin
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
