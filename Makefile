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
ifeq ($(strip $(shell git status --porcelain 2>/dev/null)),)
  GIT_TREE_STATE=clean
else
  GIT_TREE_STATE=dirty
endif
COMMIT ?= $(shell git rev-parse HEAD)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
LDFLAGS ?= -X github.com/digitalocean/csi-digitalocean/driver.version=${VERSION} -X github.com/digitalocean/csi-digitalocean/driver.commit=${COMMIT} -X github.com/digitalocean/csi-digitalocean/driver.gitTreeState=${GIT_TREE_STATE}
PKG ?= github.com/digitalocean/csi-digitalocean/cmd/do-csi-plugin

VERSION ?= $(shell cat VERSION)
DOCKER_REPO ?= digitalocean/do-csi-plugin

all: check-unused test

publish: compile build push clean

.PHONY: bump-version
bump-version:
	@[ "${NEW_VERSION}" ] || ( echo "NEW_VERSION must be set (ex. make NEW_VERSION=v1.x.x bump-version)"; exit 1 )
	@(echo ${NEW_VERSION} | grep -E "^v") || ( echo "NEW_VERSION must be a semver ('v' prefix is required)"; exit 1 )
	@echo "Bumping VERSION from $(VERSION) to $(NEW_VERSION)"
	@echo $(NEW_VERSION) > VERSION
	@cp deploy/kubernetes/releases/csi-digitalocean-${VERSION}.yaml deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e 's#digitalocean/do-csi-plugin:${VERSION}#digitalocean/do-csi-plugin:${NEW_VERSION}#g' deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@git add --intent-to-add deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	$(eval NEW_DATE = $(shell date +%Y.%m.%d))
	@sed -i'' -e 's/## unreleased/## ${NEW_VERSION} - ${NEW_DATE}/g' CHANGELOG.md
	@ echo '## unreleased\n' | cat - CHANGELOG.md > temp && mv temp CHANGELOG.md
	@rm -f deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml-e CHANGELOG.md-e

.PHONY: compile
compile:
	@echo "==> Building the project"
	@docker run --rm -it -e GOOS=${OS} -e GOARCH=amd64 -v ${PWD}/:/app -w /app golang:1.13-alpine sh -c 'apk add git && go build -mod=vendor -o cmd/do-csi-plugin/${NAME} -ldflags "$(LDFLAGS)" ${PKG}'


.PHONY: check-unused
check-unused: vendor
	@git diff --exit-code -- go.sum go.mod vendor/ || ( echo "there are uncommitted changes to the Go modules and/or vendor files -- please run 'make vendor' and commit the changes first"; exit 1 )

.PHONY: test
test:
	@echo "==> Testing all packages"
	@go test -v ./...

.PHONY: test-integration
test-integration:

	@echo "==> Started integration tests"
	@env go test -v -tags integration -count 1 ./test/...


.PHONY: build
build:
	@echo "==> Building the docker image"
	@docker build -t $(DOCKER_REPO):$(VERSION) cmd/do-csi-plugin -f cmd/do-csi-plugin/Dockerfile

.PHONY: push
push:
ifeq ($(DOCKER_REPO),digitalocean/do-csi-plugin)
  ifneq ($(BRANCH),release-0.4)
    ifneq ($(VERSION),dev)
	  $(error "Only the `dev` tag can be published from branch release-0.4")
    endif
  endif
endif
	@echo "==> Publishing $(DOCKER_REPO):$(VERSION)"
	@docker push $(DOCKER_REPO):$(VERSION)
	@echo "==> Your image is now available at $(DOCKER_REPO):$(VERSION)"

.PHONY: vendor
vendor:
	@GO111MODULE=on go mod tidy
	@GO111MODULE=on go mod vendor

.PHONY: clean
clean:
	@echo "==> Cleaning releases"
	@GOOS=${OS} go clean -i -x ./...
