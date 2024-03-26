ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GO_CMD ?= go
GO_FMT ?= gofmt
GO_TEST_FLAGS ?= -v -count=1 -timeout=60m

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

KIND = $(PROJECT_DIR)/bin/kind
.PHONY: kind
kind:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install sigs.k8s.io/kind@v0.20.0

.PHONY: setup
setup: kind
	./hack/setup.sh

test-kube-scheduler:
	$(GO_CMD) test $(GO_TEST_FLAGS) -run TestKubeScheduler ./test

test-kueue:
	$(GO_CMD) test $(GO_TEST_FLAGS) -run TestKueue ./test

test-cosheduling:
	$(GO_CMD) test $(GO_TEST_FLAGS) -run TestCoscheduling ./test
