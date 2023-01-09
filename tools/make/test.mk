# Copyright 2022 Mia srl

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#    http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

##@ Go Tests Goals

DEBUG_TEST ?=
ifeq ($(DEBUG_TEST),1)
GO_TEST_DEBUG_FLAG := -v
else
GO_TEST_DEBUG_FLAG :=
endif

ENVTEST_K8S_VERSION ?= $(shell cat $(TOOLS_DIR)/ENVTEST_K8S_VERSION)

.PHONY: test/unit
test/unit:
	echo "Running tests..."
	go test $(GO_TEST_DEBUG_FLAG) -race ./...

.PHONY: test/integration
test/integration: $(TOOLS_BIN)/setup-envtest envtest-assets
	echo "Running integration tests..."
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test $(GO_TEST_DEBUG_FLAG) --tags=integration -race ./...

.PHONY: test/coverage
test/coverage:
	echo "Running tests with coverage on..."
	go test $(GO_TEST_DEBUG_FLAG) -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: test/integration-coverage
test/integration-coverage: $(TOOLS_BIN)/setup-envtest envtest-assets
	echo "Running ci tests with coverage on..."
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test --tags=integration -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: test
test: test/unit

.PHONY: integration-test
integration-test: test/integration

.PHONY: test-coverage
test-coverage: test/coverage

.PHONY: test-integration-coverage
test-integration-coverage: test/integration-coverage

.PHONY: show-coverage
show-coverage: test-coverage
	go tool cover -func=coverage.txt

$(TOOLS_BIN)/setup-envtest: $(TOOLS_DIR)/ENVTEST_VERSION
	$(eval ENVTEST_VERSION := $(shell cat $<))
	mkdir -p $(TOOLS_BIN)
	echo "Installing testenv $(ENVTEST_VERSION) bin in $(TOOLS_BIN)..."
	GOBIN=$(TOOLS_BIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

.PHONY: envtest-assets
envtest-assets:
	echo "Setup testenv with k8s $(ENVTEST_K8S_VERSION) version..."
	$(eval KUBEBUILDER_ASSETS := $(shell $(TOOLS_BIN)/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLS_BIN) -p path))
