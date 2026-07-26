# kcp-marketplace

SHELL := /usr/bin/env bash
GO ?= go
GOBIN ?= $(CURDIR)/bin
MODULES := . ./apis

TOOLS_DIR := $(CURDIR)/hack/tools
MINDL := $(GO) tool codeberg.org/ntnn/mindl

GOLANGCI_LINT_VERSION ?= 2.12.2
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)/golangci-lint

# kcp components
KCP_VERSION ?= 0.32.0
KCP_ASSETS_DIR := $(TOOLS_DIR)/kcp-$(KCP_VERSION)
KCP_COMPONENTS := kcp kcp-front-proxy cache-server
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
	$(GO) build -o $(GOBIN)/access-vws ./cmd/access-vws
	$(GO) build -o $(GOBIN)/apiexport-vws ./cmd/apiexport-vws

.PHONY: test
test: ## Unit tests, no cluster.
	@set -e; for m in $(MODULES); do (cd $$m && $(GO) test ./...); done

.PHONY: test-integration
test-integration: $(KCP) ## Integration tests against in-process sharded kcp (CI gate).
	TEST_KCP_ASSETS=$(KCP_ASSETS_DIR) $(GO) test -tags=integration ./test/...

.PHONY: tools
tools: $(GOLANGCI_LINT) $(KCP_BINARIES) ## Pull all pinned tool artefacts via mindl.

$(GOLANGCI_LINT):
	mkdir -p $(dir $@)
	$(MINDL) download -tool golangci-lint -out $@ -version $(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint over all modules.
	@set -e; for m in $(MODULES); do (cd $$m && $(GOLANGCI_LINT) run); done

.PHONY: tidy
tidy: ## go mod tidy all modules.
	@set -e; for m in $(MODULES); do (cd $$m && $(GO) mod tidy); done

.PHONY: codegen
codegen: ## Regenerate deepcopy and other in-tree generated code.
	hack/update-codegen.sh

.PHONY: manifests
manifests: ## Regenerate committed manifests into config/.
	hack/update-manifests.sh

.PHONY: ui
ui: ## Build the SPA.
	cd ui && npm ci && npm run build

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
