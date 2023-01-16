# Copyright Mia srl
# SPDX-License-Identifier: Apache-2.0

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#    http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

##@ Go Builds Goals

ifeq ($(IS_LIBRARY), 1)

BUILD_DATE:= $(shell date -u "+%Y-%m-%d")
GO_LDFLAGS += -s -w

ifdef VERSION_MODULE_NAME
GO_LDFLAGS += -X $(VERSION_MODULE_NAME).Version=$(VERSION)
GO_LDFLAGS += -X $(VERSION_MODULE_NAME).BuildDate=$(BUILD_DATE)
endif

.PHONY: go/build
go/build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)"  $(BUILD_PATH)

# By default run the build for the host machine only
.PHONY: build
build: go/build

else

.PHONY: go/build
go/build:
	$(TOOLS_BIN)/goreleaser build --single-target --snapshot --rm-dist

.PHONY: go/build/multiarch
go/build/multiarch:
	$(TOOLS_BIN)/goreleaser build --snapshot --rm-dist

.PHONY: build-deps
build-deps:

$(TOOLS_BIN)/goreleaser: $(TOOLS_DIR)/GORELEASER_VERSION
	$(eval GORELEASER_VERSION:= $(shell cat $<))
	mkdir -p $(TOOLS_BIN)
	$(info Installing goreleaser $(GORELEASER_VERSION) bin in $(TOOLS_BIN))
	GOBIN=$(TOOLS_BIN) go install github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)

build-deps: $(TOOLS_BIN)/goreleaser

# By default run the build for the host machine only
.PHONY: build
build: $(TOOLS_BIN)/goreleaser go/build

.PHONY: build-multiarch
build-multiarch: $(TOOLS_BIN)/goreleaser go/build/multiarch

endif
