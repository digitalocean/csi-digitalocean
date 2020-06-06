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

NAME=do-csi-plugin
OS ?= linux
GO_VERSION := 1.14
ifeq ($(strip $(shell git status --porcelain 2>/dev/null)),)
  GIT_TREE_STATE=clean
else
  GIT_TREE_STATE=dirty
endif
COMMIT ?= $(shell git rev-parse HEAD)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
LDFLAGS ?= -X github.com/digitalocean/csi-digitalocean/driver.version=${VERSION} -X github.com/digitalocean/csi-digitalocean/driver.commit=${COMMIT} -X github.com/digitalocean/csi-digitalocean/driver.gitTreeState=${GIT_TREE_STATE}
PKG ?= github.com/digitalocean/csi-digitalocean/cmd/do-csi-plugin
ifneq ($(VERSION),)
  VERSION := $(shell /bin/echo -n "$(VERSION)" | tr -c '[:alnum:]._-' '-')
else
  VERSION ?= $(shell cat VERSION)
endif
KUBERNETES_VERSION ?= 1.18.3
DOCKER_REPO ?= digitalocean/do-csi-plugin
CANONICAL_RUNNER_IMAGE = digitalocean/k8s-e2e-test-runner
RUNNER_IMAGE ?= $(CANONICAL_RUNNER_IMAGE)

# Max Volumes to a Single Droplet is 7
INTEGRATION_PARALLEL ?= 7

ifneq ($(RUNNER_IMAGE_TAG_PREFIX),)
  RUNNER_IMAGE_TAG_PREFIX := $(shell /bin/echo -n "$(RUNNER_IMAGE_TAG_PREFIX)" | tr -c '[:alnum:]._-' '-')
	RUNNER_IMAGE_TAG_PREFIX := $(RUNNER_IMAGE_TAG_PREFIX)-
endif

all: check-unused test

publish: compile build push clean

.PHONY: update-k8s
update-k8s:
	env KUBERNETES_VERSION=$(KUBERNETES_VERSION) scripts/update-k8s.sh

.PHONY: bump-version
bump-version:
	@[ "${NEW_VERSION}" ] || ( echo "NEW_VERSION must be set (ex. make NEW_VERSION=v1.x.x bump-version)"; exit 1 )
	@(echo ${NEW_VERSION} | grep -E "^v") || ( echo "NEW_VERSION must be a semver ('v' prefix is required)"; exit 1 )
	@echo "Bumping VERSION from $(VERSION) to $(NEW_VERSION)"
	@echo $(NEW_VERSION) > VERSION
	@cp deploy/kubernetes/releases/csi-digitalocean-latest.yaml deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e 's#digitalocean/do-csi-plugin:dev#digitalocean/do-csi-plugin:${NEW_VERSION}#g' deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@git add --intent-to-add deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e '/^# This file is only for development use/d' deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	$(eval NEW_DATE = $(shell date +%Y.%m.%d))
	@sed -i'' -e 's/## unreleased/## ${NEW_VERSION} - ${NEW_DATE}/g' CHANGELOG.md
	@ echo '## unreleased\n' | cat - CHANGELOG.md > temp && mv temp CHANGELOG.md
	@rm -f deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml-e CHANGELOG.md-e

.PHONY: compile
compile:
	@echo "==> Building the project"
	@docker run --rm -e GOOS=${OS} -e GOARCH=amd64 -v ${PWD}/:/app -w /app golang:${GO_VERSION}-alpine sh -c 'apk add git && go build -mod=vendor -o cmd/do-csi-plugin/${NAME} -ldflags "$(LDFLAGS)" ${PKG}'

.PHONY: check-unused
check-unused: vendor
	@git diff --exit-code -- go.sum go.mod vendor/ || ( echo "there are uncommitted changes to the Go modules and/or vendor files -- please run 'make vendor' and commit the changes first"; exit 1 )

.PHONY: test
test:
	@echo "==> Testing all packages"
	@GO111MODULE=on go test -mod=vendor -v ./...

.PHONY: test-integration
test-integration:
	@echo "==> Started integration tests"
	@env go test -parallel ${INTEGRATION_PARALLEL} -count 1 -v -tags integration ./test/...

.PHONY: test-e2e
test-e2e:
	@echo "==> Started end-to-end tests"
	@GO111MODULE=on GOFLAGS=-mod=vendor ./test/e2e/e2e.sh $(E2E_ARGS)

.PHONY: build
build:
	@echo "==> Building the docker image"
	@docker build -t $(DOCKER_REPO):$(VERSION) cmd/do-csi-plugin -f cmd/do-csi-plugin/Dockerfile

.PHONY: push
push:
ifeq ($(DOCKER_REPO),digitalocean/do-csi-plugin)
  ifneq ($(BRANCH),master)
    ifneq ($(VERSION),dev)
	  $(error "Only the `dev` tag can be published from non-master branches")
    endif
  endif
endif
	@echo "==> Publishing $(DOCKER_REPO):$(VERSION)"
	@docker push $(DOCKER_REPO):$(VERSION)
	@echo "==> Your image is now available at $(DOCKER_REPO):$(VERSION)"

# runner-build builds the e2e test runner image. Sadly, this is much harder to
# do cache-efficiently than it should be for Docker multi-stage builds since
# those require build targets to be pushed and referenced individually. See
# https://andrewlock.net/caching-docker-layers-on-serverless-build-hosts-with-multi-stage-builds---target,-and---cache-from/
# for more context.
#
# The Makefile target implementation does a lot of pulling and employs
# --cache-from heavily to ensure building the image works as fast as possible.
# In particular, building the layers that compile the individual Kubernetes
# releases can take a fairly long time otherwise.
#
# The build uses the following, customizable variables:
#
# - RUNNER_IMAGE: Overwrite the runner image to be build and pushed.
# - RUNNER_IMAGE_TAG_PREFIX: A prefix before the tag. This is to allow building
#       images specific to PRs during CI, using the remote branch name as the
#       prefix. For master builds during CI, the prefix will be left empty.
#
# CANONICAL_RUNNER_IMAGE is not overwriteable; it references the canonical
# runner image name as a cache source only.
.PHONY: runner-build
runner-build:
	@echo "pulling cache images"
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):builder || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):tests-1.17 || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):tests-1.16 || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15 || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):tests-1.15 || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.14 || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):tests-1.14 || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tools || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):tools || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)runtime || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):runtime || true
	@docker pull $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)latest || true
	@docker pull $(CANONICAL_RUNNER_IMAGE):latest || true

	@echo "building target builder-pre-1.16"
	@docker build --target builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 -f test/e2e/Dockerfile test/e2e

	@echo "building target builder"
	@docker build --target builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder -f test/e2e/Dockerfile test/e2e

	@echo "building target tests-1.17"
	@docker build --target tests-1.17 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.17 \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 -f test/e2e/Dockerfile test/e2e

	@echo "building target tests-1.16"
	@docker build --target tests-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.17 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.16 \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 -f test/e2e/Dockerfile test/e2e

	@echo "building target tests-1.15"
	@docker build --target tests-1.15 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.17 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.15 \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15 -f test/e2e/Dockerfile test/e2e

	@echo "building target tests-1.14"
	@docker build --target tests-1.14 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.17 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.15 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.14 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.14 \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.14 -f test/e2e/Dockerfile test/e2e

	@echo "building target tools"
	@docker build --target tools \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.17 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.15 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.14 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.14 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tools \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tools \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tools -f test/e2e/Dockerfile test/e2e

	@echo "building target runtime"
	@docker build --target runtime \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.17 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.15 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.14 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.14 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tools \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tools \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)runtime \
		--cache-from $(CANONICAL_RUNNER_IMAGE):runtime \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)runtime -f test/e2e/Dockerfile test/e2e

	@echo "building final image"
	@docker build \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder-pre-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder \
		--cache-from $(CANONICAL_RUNNER_IMAGE):builder \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.17 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.16 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.15 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.14 \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tests-1.14 \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tools \
		--cache-from $(CANONICAL_RUNNER_IMAGE):tools \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)runtime \
		--cache-from $(CANONICAL_RUNNER_IMAGE):runtime \
		--cache-from $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)latest \
		--cache-from $(CANONICAL_RUNNER_IMAGE):latest \
		-t $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)latest -f test/e2e/Dockerfile test/e2e

runner-push: runner-build
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder-pre-1.16
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)builder
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.17
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.16
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.15
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tests-1.14
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)tools
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)runtime
	@docker push $(RUNNER_IMAGE):$(RUNNER_IMAGE_TAG_PREFIX)latest

.PHONY: vendor
vendor:
	@GO111MODULE=on go mod tidy
	@GO111MODULE=on go mod vendor

.PHONY: clean
clean:
	@echo "==> Cleaning releases"
	@GOOS=${OS} go clean -i -x ./...
