PKG=github.com/cyverse/irods-mcp-server
VERSION=v$(shell jq -r .version package_info.json)
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X '${PKG}/common.serverVersion=${VERSION}' -X '${PKG}/common.gitCommit=${GIT_COMMIT}' -X '${PKG}/common.buildDate=${BUILD_DATE}'"
GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)
OS_NAME:=$(shell grep -E '^ID=' /etc/os-release | cut -d'=' -f2 | tr -d '"')
SHELL:=/bin/sh
ADDUSER_FLAGS:=
ifeq (${OS_NAME},centos)
	ADDUSER_FLAGS=-r -d /dev/null -s /sbin/nologin 
else ifeq (${OS_NAME},almalinux)
	ADDUSER_FLAGS=-r -d /dev/null -s /sbin/nologin 
else ifeq (${OS_NAME},ubuntu)
	ADDUSER_FLAGS=--system --no-create-home --shell /sbin/nologin --group
else 
	ADDUSER_FLAGS=--system --no-create-home --shell /sbin/nologin --group
endif

DOCKER_IMAGE=cyverse/irods-mcp-server
DOCKERFILE=docker/Dockerfile
USER_ID=$(shell id -u)

.PHONY: build
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/irods-mcp-server ./cmd/main.go

.PHONY: image
image:
	docker build -t $(DOCKER_IMAGE):${VERSION} -f $(DOCKERFILE) .
	docker tag $(DOCKER_IMAGE):${VERSION} $(DOCKER_IMAGE):latest

.PHONY: push
push: image
	docker push $(DOCKER_IMAGE):${VERSION}
	docker push $(DOCKER_IMAGE):latest

.PHONY: release
release: build
	mkdir -p release
	mkdir -p release/bin
	cp bin/irods-mcp-server release/bin
	mkdir -p release/install
	cp install/config.yaml release/install
	cp install/irods-mcp-server.service release/install
	cp install/README.md release/install
	cp Makefile.release release/Makefile
	cd release && tar zcvf ../irods-mcp-server.tar.gz *

.PHONY: checkroot
checkroot:
	@echo "Checking for root privileges..."
	@if [ -z "${USER_ID}" ]; then \
		echo "Error: Unable to determine user ID."; \
		exit 1; \
	elif [ "${USER_ID}" -ne 0 ]; then \
		echo "Error: This target requires root privileges."; \
        echo "Please run with sudo."; \
		exit 1; \
	fi; \
	echo "Running with root privileges (OK)."

.PHONY: install
install: checkroot
	cp bin/irods-mcp-server /usr/bin
	cp install/irods-mcp-server.service /usr/lib/systemd/system/
	id -u irodsmcp > /dev/null 2>&1 || adduser ${ADDUSER_FLAGS} irodsmcp
	mkdir -p /etc/irods-mcp-server
	cp install/config.yaml /etc/irods-mcp-server
	chown irodsmcp:irodsmcp /etc/irods-mcp-server/config.yaml
	chmod 660 /etc/irods-mcp-server/config.yaml

.PHONY: uninstall
uninstall:
	rm -f /usr/bin/irods-mcp-server
	rm -f /etc/systemd/system/multi-user.target.wants/irods-mcp-server.service || true
	rm -f /usr/lib/systemd/system/irods-mcp-server.service
	userdel irodsmcp || true
	groupdel irodsmcp || true
	rm -rf /etc/irods-mcp-server
