VERSION ?= v0.1.0
NAME=do-csi-plugin

all: publish

publish: compile build push clean

compile:
	@echo "==> Building the project"
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${NAME}

build:
	@echo "==> Building the docker image"
	@docker build -t digitalocean/do-csi-plugin:$(VERSION) .

push:
	@echo "==> Publishing digitalocean/do-csi-plugin:$(VERSION)"
	@docker push digitalocean/do-csi-plugin:$(VERSION)
	@echo "==> Your image is now available at digitalocean/do-csi-plugin:$(VERSION)"

clean:
	@echo "==> Cleaning releases"
	@GOOS=linux go clean -i -x ./...

.PHONY: all push fetch build-image clean
