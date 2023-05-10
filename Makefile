GO ?= go

REGISTRY ?= ccr.ccs.tencentyun.com/tkeedge
SCHEDULERIMAGES ?= orin-device-scheduler
DEVICEIMAGES ?= orin-device-plugin

PLATFORM ?= linux/arm64,linux/amd64,linux/arm

COMMA:= ,
EMPTY:=
SPACE:= $(EMPTY) $(EMPTY)
LINE:= /
DOT:= .

ifeq ($(origin VERSION), undefined)
VERSION := $(shell git describe --match 'v[0-9]*' --dirty='.m' --always)
endif
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)
PKG=github.com/superedge/orin-device-system

GO_LDFLAGS=-ldflags '-X $(PKG)/main.Version=$(VERSION) -X $(EXTRA_LDFLAGS)'

scheduler.image.build:
	docker buildx build --platform $(PLATFORM) --build-arg GO_LDFLAGS="$(GO_LDFLAGS)" -f ./build/scheduler/Dockerfile -t $(REGISTRY)/$(SCHEDULERIMAGES):$(VERSION) .

scheduler.image.push:
	docker buildx build --push --platform $(PLATFORM) -f Dockerfile -t $(REGISTRY)/$(SCHEDULERIMAGES):$(VERSION) .

scheduler.binary.build: ## build the go packages
	@echo "$(WHALE) $@"
	@$(GO) build ${EXTRA_FLAGS} ${GO_LDFLAGS}


device.image.build:
	docker buildx build --platform $(PLATFORM) --build-arg GO_LDFLAGS="$(GO_LDFLAGS)" -f /build/device-plugin/Dockerfile -t $(REGISTRY)/$(DEVICEIMAGES):$(VERSION) .

device.image.push:
	docker buildx build --push --platform $(PLATFORM) -f Dockerfile -t $(REGISTRY)/$(DEVICEIMAGES):$(VERSION) .

device.binary.build: ## build the go packages
	@echo "$(WHALE) $@"
	@$(GO) build ${EXTRA_FLAGS} ${GO_LDFLAGS}
