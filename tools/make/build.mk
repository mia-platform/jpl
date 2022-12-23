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

##@ Go Builds Goals

BUILD_DATE := $(shell date -u "+%Y-%m-%d")

ifdef VERSION_MODULE_NAME
GO_LDFLAGS += -X $(VERSION_MODULE_NAME).Version=$(VERSION)
GO_LDFLAGS += -X $(VERSION_MODULE_NAME).BuildDate=$(BUILD_DATE)
endif

# one day I will find a nicer way to handle the arm versions but this is not the day...
NORMALIZED_SUPPORTED_PLATFORMS := $(subst v6,6,$(subst v7,7,$(SUPPORTED_PLATFORMS)))

.PHONY: build/%
build/%:
	$(eval OS := $(word 1,$(subst /, ,$*)))
	$(eval ARCH := $(word 2,$(subst /, ,$*)))
	$(eval ARM := $(word 3,$(subst /, ,$*)))
	echo "Building for ${OS} ${ARCH} ${ARM}"

	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) GOARM=$(ARM) go build -ldflags "$(GO_LDFLAGS)" \
		-o $(OUTPUT_DIR)/$*/$(CMDNAME) $(BUILD_PATH)

# By default run the build for the host machine only
.PHONY: build
build: build/$(GOOS)/$(GOARCH)/$(GOARM)

.PHONY: build-multiarch
build-multiarch: $(foreach PLATFORM,$(NORMALIZED_SUPPORTED_PLATFORMS),$(addprefix build/, $(PLATFORM)))
