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

## Bump the version in the version file. Set BUMP to [ patch | major | minor ]
BUMP := patch
VERSION ?= $(shell cat VERSION)

all: test

publish: compile build push clean

.PHONY: bump-version
bump-version: 
	@go get -u github.com/jessfraz/junk/sembump # update sembump tool
	$(eval NEW_VERSION = $(shell sembump --kind $(BUMP) $(VERSION)))
	@echo "Bumping VERSION from $(VERSION) to $(NEW_VERSION)"
	@echo $(NEW_VERSION) > VERSION
	@cp deploy/kubernetes/releases/csi-digitalocean-${VERSION}.yaml deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e 's/${VERSION}/${NEW_VERSION}/g' deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml
	@sed -i'' -e 's/${VERSION}/${NEW_VERSION}/g' README.md
	$(eval NEW_DATE = $(shell date +%Y.%m.%d))
	@sed -i'' -e 's/## unreleased/## ${NEW_VERSION} - ${NEW_DATE}/g' CHANGELOG.md 
	@ echo '## unreleased\n' | cat - CHANGELOG.md > temp && mv temp CHANGELOG.md
	@rm README.md-e CHANGELOG.md-e deploy/kubernetes/releases/csi-digitalocean-${NEW_VERSION}.yaml-e

.PHONY: compile
compile:
	@echo "==> Building the project"
	@env CGO_ENABLED=0 GOOS=${OS} GOARCH=amd64 go build -o cmd/do-csi-plugin/${NAME} -ldflags "$(LDFLAGS)" ${PKG} 


.PHONY: test
test:
	@echo "==> Testing all packages"
	@go test -v ./...


.PHONY: build
build:
	@echo "==> Building the docker image"
	@docker build -t digitalocean/do-csi-plugin:$(VERSION) cmd/do-csi-plugin -f cmd/do-csi-plugin/Dockerfile

.PHONY: push
push:
ifeq ($(shell [[ $(BRANCH) != "master" && $(VERSION) != "dev" ]] && echo true ),true)
	@echo "ERROR: Publishing image with a SEMVER version '$(VERSION)' is only allowed from master"
else
	@echo "==> Publishing digitalocean/do-csi-plugin:$(VERSION)"
	@docker push digitalocean/do-csi-plugin:$(VERSION)
	@echo "==> Your image is now available at digitalocean/do-csi-plugin:$(VERSION)"
endif

.PHONY: clean
clean:
	@echo "==> Cleaning releases"
	@GOOS=${OS} go clean -i -x ./...
