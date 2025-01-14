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

### Kubernetes 1.32
FROM builder As tests-1.32
ARG KUBE_VERSION_1_32=1.32.0
ARG KUBE_VERSION_1_32_E2E_BIN_SHA256_CHECKSUM=742dee5fdbab14202db14142b1387c13076421666383bd6259350899b049c2c1

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_32}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_32_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.32.test
RUN cp ginkgo /ginkgo-1.32

### Kubernetes 1.31
FROM builder As tests-1.31
ARG KUBE_VERSION_1_31=1.31.0
ARG KUBE_VERSION_1_31_E2E_BIN_SHA256_CHECKSUM=aa9747ca53660a3457c5158fa47ed6a5f7fb7a82fa50c0f05dbeb607f579a7e8

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_31}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_31_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.31.test
RUN cp ginkgo /ginkgo-1.31

### Kubernetes 1.30
FROM builder As tests-1.30
ARG KUBE_VERSION_1_30=1.30.0
ARG KUBE_VERSION_1_30_E2E_BIN_SHA256_CHECKSUM=6f6f0acf7935197b47a69c56a4e14d4ac7edc78054372da0d86eb601bebd308c

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_30}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_30_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.30.test
RUN cp ginkgo /ginkgo-1.30

### Kubernetes 1.29
FROM builder As tests-1.29
ARG KUBE_VERSION_1_29=1.29.0
ARG KUBE_VERSION_1_29_E2E_BIN_SHA256_CHECKSUM=b0d19ce356dce19eeb6d36bba99545ca334d0964ac68f1088ec0ee68cfcee530

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_29}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_29_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.29.test
RUN cp ginkgo /ginkgo-1.29

### Kubernetes 1.28
FROM builder AS tests-1.28
ARG KUBE_VERSION_1_28=1.28.0
ARG KUBE_VERSION_1_28_E2E_BIN_SHA256_CHECKSUM=807f5ce07395f01e916873847a553dc32d437341695360bbcad5932e9fed094e

RUN curl --fail --location https://dl.k8s.io/v${KUBE_VERSION_1_28}/kubernetes-test-linux-amd64.tar.gz | tar xvzf - --strip-components 3 kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo
RUN echo "${KUBE_VERSION_1_28_E2E_BIN_SHA256_CHECKSUM}" e2e.test | sha256sum --check
RUN cp e2e.test /e2e.1.28.test
RUN cp ginkgo /ginkgo-1.28

FROM golang:1.23 AS tools
# See comment at the bottom on why we need tini.
ARG TINI_VERSION=0.19.0
# doctl is needed to support clusters that had their kubeconfig fetched via the
# CLI because those leverage a kubeconfig authentication plugin based on doctl.
ARG DOCTL_VERSION=1.120.1

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
COPY --from=tests-1.32 /e2e.1.32.test /
COPY --from=tests-1.32 /ginkgo-1.32 /usr/local/bin
COPY --from=tests-1.31 /e2e.1.31.test /
COPY --from=tests-1.31 /ginkgo-1.31 /usr/local/bin
COPY --from=tests-1.30 /e2e.1.30.test /
COPY --from=tests-1.30 /ginkgo-1.30 /usr/local/bin
COPY --from=tests-1.29 /e2e.1.29.test /
COPY --from=tests-1.29 /ginkgo-1.29 /usr/local/bin
COPY --from=tests-1.28 /e2e.1.28.test /
COPY --from=tests-1.28 /ginkgo-1.28 /usr/local/bin
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
