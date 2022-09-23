# Copyright 2022 Mia-Platform

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#    http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# General variables

# Set Output Directory Path
PROJECT_DIR := $(shell pwd -P)
TOOLS_DIR := $(PROJECT_DIR)/tools
TOOLS_BIN := $(TOOLS_DIR)/bin

ENVTEST_K8S_VERSION?=1.24
GOLANCI_LINT_VERSION=1.49.0
ENVTEST_VERSION=latest

##@ Test

TEST_VERBOSE ?= "false"
.PHONY: test
test: envtest-dep envtest-assets
ifneq ($(TEST_VERBOSE), "false")
	@KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test -test.v ./...
else
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test ./...
endif

.PHONY: test-coverage
test-coverage: envtest-dep envtest-assets
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test ./... -race -coverprofile=coverage.xml -covermode=atomic

envtest-assets:
	$(eval KUBEBUILDER_ASSETS := $(shell $(TOOLS_BIN)/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLS_BIN) -p path))

.PHONY: show-coverage
show-coverage: test-coverage
	go tool cover -func=coverage.xml

##@ Clean project

.PHONY: clean
clean:
	@echo "Clean all artifact files..."
	@rm -fr coverage.xml

.PHONY: clean-go
clean-go:
	@echo "Clean golang cache..."
	@go clean -cache

.PHONY: clean-tools
clean-tools:
	@echo "Clean tools folder..."
	@[ -d tools/bin/k8s ] && chmod +w tools/bin/k8s/* || true
	@rm -fr ./tools/bin

.PHONY: clean-all
clean-all: clean clean-go clean-tools

##@ Lint

MODE ?= "colored-line-number"

.PHONY: lint
lint: lint-mod lint-vet lint-ci

lint-ci: lintgo-dep
	@echo "Linting go files..."
	$(TOOLS_BIN)/golangci-lint run --out-format=$(MODE) --config=$(TOOLS_DIR)/.golangci.yml

lint-fix: lintgo-dep
	@echo "Run lint with fix flag..."
	$(TOOLS_BIN)/golangci-lint run --config=$(TOOLS_DIR)/.golangci.yml --fix

lint-mod:
	@echo "Run go mod tidy"
	@go mod tidy -compat=1.18
## ensure all changes have been committed
	git diff --exit-code -- go.mod
	git diff --exit-code -- go.sum

lint-vet:
	@echo "Run go vet"
	@go vet ./...

##@ Dependencies

.PHONY: install-dep
install-dep: lintgo-dep envtest-dep

lintgo-dep:
	@GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(GOLANCI_LINT_VERSION)

envtest-dep:
	@GOBIN=$(TOOLS_BIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)
