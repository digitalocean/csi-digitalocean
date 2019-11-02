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

all: test

publish: compile build push clean

.PHONY: bump-version
bump-version:
	@[ "${NEW_VERSION}" ] || ( echo "NEW_VERSION must be set (ex. make NEW_VERSION=v1.x.x bump-version)"; exit 1 )
	@(echo ${NEW_VERSION} | grep -E "^v") || ( echo "NEW_VERSION must be a semver ('v' prefix is required)"; exit 1 )
	@echo "Bumping VERSION from $(VERSION) to $(NEW_VERSION)"
	@echo $(NEW_VERSION) > VERSION
	@cp deploy/kubernetes/releases/csi-digitalocean-latest.yaml deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e 's#digitalocean/do-csi-plugin:dev#digitalocean/do-csi-plugin:${NEW_VERSION}#g' deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e '/^# This file is only for development use/d' deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e 's/${VERSION}/${NEW_VERSION}/g' README.md
	$(eval NEW_DATE = $(shell date +%Y.%m.%d))
	@sed -i'' -e 's/## unreleased/## ${NEW_VERSION} - ${NEW_DATE}/g' CHANGELOG.md
	@ echo '## unreleased\n' | cat - CHANGELOG.md > temp && mv temp CHANGELOG.md

.PHONY: compile
compile:
	@echo "==> Building the project"
	@docker run --rm -it -e GOOS=${OS} -e GOARCH=amd64 -v ${PWD}/:/app -w /app golang:1.13-alpine sh -c 'apk add git && go build -o cmd/do-csi-plugin/${NAME} -ldflags "$(LDFLAGS)" ${PKG}'

.PHONY: test
test:
	@echo "==> Testing all packages"
	@go test -v ./...

.PHONY: test-integration
test-integration:
	@echo "==> Started integration tests"
	@env go test -count 1 -v -tags integration ./test/...

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

.PHONY: clean
clean:
	@echo "==> Cleaning releases"
	@GOOS=${OS} go clean -i -x ./...
