.ONESHELL:
SHA := $(shell git rev-parse --short=8 HEAD)
GITVERSION := $(shell git describe --long --all)
BUILDDATE := $(shell date -Iseconds)
VERSION := $(or ${VERSION},devel)
GO := go
GOSRC = $(shell find . -not \( -path vendor -prune \) -type f -name '*.go')

export GOPROXY := https://gomods.fi-ts.io
export GO111MODULE := on
export CGO_ENABLED := 0

BINARY := metal-cloud-controller-manager
MAINMODULE := github.com/metal-pod/metal-ccm

.PHONY: all
all:: bin/$(BINARY);

bin/$(BINARY): $(GOSRC)
	$(GO) build \
		-tags netgo \
		-ldflags \
			"-X 'github.com/metal-pod/v.Version=$(VERSION)' \
			-X 'github.com/metal-pod/v.Revision=$(GITVERSION)' \
			-X 'github.com/metal-pod/v.Gitsha1=$(SHA)' \
			-X 'github.com/metal-pod/v.Builddate=$(BUILDDATE)'" \
		-o bin/$(BINARY) \
		$(MAINMODULE)
	strip bin/$(BINARY)

.PHONY: clean
clean:
	rm -f bin/$(BINARY)

.PHONY: dockerimage
dockerimage:
	docker build -t metal-pod/metal-ccm .