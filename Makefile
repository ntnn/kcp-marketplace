# kcp-marketplace

SHELL := /usr/bin/env bash
GO ?= go
GOBIN ?= $(CURDIR)/bin
MODULES := . ./apis

TOOLS_DIR := $(CURDIR)/hack/tools
MINDL := $(GO) tool codeberg.org/ntnn/mindl

VERSION ?= $(shell git describe --tags --abbrev=0 --match 'v*')
COMMIT ?= $(shell git rev-parse --short HEAD)
REGISTRY ?= ghcr.io/ntnn/kcp-marketplace
VWS_IMG ?= $(REGISTRY)/marketplace-vws:$(VERSION)
UI_IMG ?= $(REGISTRY)/ui:$(VERSION)

GOLANGCI_LINT_VERSION ?= 2.12.2
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)/golangci-lint

CONTROLLER_GEN_VERSION ?= 0.19.0
CONTROLLER_GEN := $(TOOLS_DIR)/controller-gen-$(CONTROLLER_GEN_VERSION)/controller-gen

# kcp components
KCP_VERSION ?= 0.32.0
KCP_ASSETS_DIR := $(TOOLS_DIR)/kcp-$(KCP_VERSION)
KCP_COMPONENTS := kcp kcp-front-proxy cache-server sharded-test-server
KCP_BINARIES := $(addprefix $(KCP_ASSETS_DIR)/,$(KCP_COMPONENTS))
KCP := $(KCP_ASSETS_DIR)/kcp

# $(1) = component name (release archive base and in-archive bin/<name>).
define mindl_kcp_component
$(KCP_ASSETS_DIR)/$(1):
	mkdir -p $$(dir $$@)
	$(MINDL) download -common -out $$@ \
		-url 'https://github.com/kcp-dev/kcp/releases/download/v{{.Version}}/$(1)_{{.Version}}_{{.OS}}_{{.Arch}}.tar.gz' \
		-inarchive 'bin/$(1)' \
		-version $(KCP_VERSION)
endef
$(foreach c,$(KCP_COMPONENTS),$(eval $(call mindl_kcp_component,$(c))))

.PHONY: all
all: lint test build

.PHONY: build
build: ## Compile binaries into bin/.
	$(GO) build -o $(GOBIN)/marketplace-vws ./cmd/marketplace-vws

.PHONY: test
test: ## Unit tests, no cluster.
	@set -e; for m in $(MODULES); do (cd $$m && $(GO) test -race ./...); done

.PHONY: test-e2e
test-e2e: $(KCP_BINARIES) ## Integration tests against in-process sharded kcp (CI gate).
	KCP_ASSET_SHARDED_TEST_SERVER=$(KCP_ASSETS_DIR)/sharded-test-server \
	KCP_ASSET_KCP=$(KCP_ASSETS_DIR)/kcp \
	KCP_ASSET_KCP_FRONT_PROXY=$(KCP_ASSETS_DIR)/kcp-front-proxy \
	KCP_ASSET_CACHE_SERVER=$(KCP_ASSETS_DIR)/cache-server \
	NO_GORUN=1 \
	$(GO) test -race -tags=e2e ./test/e2e/...

.PHONY: tools
tools: $(GOLANGCI_LINT) $(CONTROLLER_GEN) $(KCP_BINARIES) ## Pull all pinned tool artefacts via mindl.

$(GOLANGCI_LINT):
	mkdir -p $(dir $@)
	$(MINDL) download -tool golangci-lint -out $@ -version $(GOLANGCI_LINT_VERSION)

$(CONTROLLER_GEN):
	mkdir -p $(dir $@)
	$(MINDL) download -common -out $@ \
		-url 'https://github.com/kubernetes-sigs/controller-tools/releases/download/v{{.Version}}/controller-gen-{{.OS}}-{{.Arch}}{{.Exe}}' \
		-version $(CONTROLLER_GEN_VERSION)

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint over all modules.
	@set -e; for m in $(MODULES); do (cd $$m && $(GOLANGCI_LINT) run); done

.PHONY: tidy
tidy: ## go mod tidy all modules.
	@set -e; for m in $(MODULES); do (cd $$m && $(GO) mod tidy); done

.PHONY: codegen
codegen: $(CONTROLLER_GEN) ## Regenerate deepcopy and other in-tree generated code.
	CONTROLLER_GEN=$(CONTROLLER_GEN) hack/update-codegen.sh

.PHONY: manifests
manifests: ## Regenerate committed manifests into config/.
	hack/update-manifests.sh

.PHONY: ui
ui: ## Build the SPA.
	cd ui && npm ci && npm run build

.PHONY: test-ui-e2e
test-ui-e2e: ## Run playwright tests against dev.
	cd ui && npm ci && npm run test:e2e:live

.PHONY: docker-build-vws
docker-build-vws: ## Build the marketplace-vws image.
	docker build -f cmd/marketplace-vws/Dockerfile --build-arg VERSION=$(VERSION) -t $(VWS_IMG) .

.PHONY: docker-build-ui
docker-build-ui: ## Build the ui image.
	docker build -t $(UI_IMG) ui

.PHONY: docker-build
docker-build: docker-build-vws docker-build-ui ## Build both production images.

.PHONY: docker-push
docker-push: ## Push both production images.
	docker push $(VWS_IMG)
	docker push $(UI_IMG)

.PHONY: ocm-build
ocm-build: ## Build the OCM component version into ./transport-archive.
	VERSION=$(VERSION) COMMIT=$(COMMIT) ocm add cv -c component-constructor.yaml -r ./transport-archive

.PHONY: ocm-transfer
ocm-transfer: ## Transfer the OCM component version from ./transport-archive to upstream
	ocm get cv --output json ./transport-archive// \
		| yq '.[] | .component.name + ":" + .component.version' \
		| while read descriptor; do \
			ocm transfer cv --recursive \
				./transport-archive//$$descriptor \
				ghcr.io/ntnn/kcp-marketplace; \
	done

.PHONY: up
up: ## Stand up the full kind stack for hands-on testing.
	hack/local-up.sh

.PHONY: down
down: ## Tear down the kind stack.
	hack/local-down.sh

.PHONY: clean
clean:
	rm -rf $(GOBIN)

.PHONY: help
help:
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'
