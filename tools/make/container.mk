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

##@ Docker Images Goals

# Force enable buildkit as a build engine
DOCKER_CMD := DOCKER_BUILDKIT=1 docker
TAG ?= $(shell git describe --tags 2>/dev/null || echo latest)
# Making the subst function works with spaces and comas required this hack
SPACE := $() $()
COMA := ,
DOCKER_SUPPORTED_PLATFORMS := $(subst $(SPACE),$(COMA),$(SUPPORTED_PLATFORMS))
IMAGE_TAGS := $(foreach REGISTRY,$(CONTAINER_REGISTRIES),$(addprefix --tag , $(REGISTRY)/$(CMDNAME):$(TAG)))
CONTAINER_BUILD_DATE := $(shell date -u "+%Y-%m-%dT%H:%M:%SZ")

DOCKER_LABELS := --label "org.opencontainers.image.title=$(CMDNAME)"
DOCKER_LABELS += --label "org.opencontainers.image.description=$(DESCRIPTION)"
DOCKER_LABELS += --label "org.opencontainers.image.url=$(SOURCE_URL)"
DOCKER_LABELS += --label "org.opencontainers.image.source=$(SOURCE_URL)"
DOCKER_LABELS += --label "org.opencontainers.image.version=$(TAG)"
DOCKER_LABELS += --label "org.opencontainers.image.created=$(CONTAINER_BUILD_DATE)"
DOCKER_LABELS += --label "org.opencontainers.image.revision=$(shell git rev-parse HEAD 2>/dev/null)"
DOCKER_LABELS += --label "org.opencontainers.image.licenses=$(LICENSE)"
DOCKER_LABELS += --label "org.opencontainers.image.documentation=$(DOCUMENTATION_URL)"
DOCKER_LABELS += --label "org.opencontainers.image.vendor=$(VENDOR_NAME)"

.PHONY: docker/build/%
docker/build/%:
	$(eval OS := $(word 1,$(subst ., ,$*)))
	$(eval ARCH := $(word 2,$(subst ., ,$*)))
	$(eval ARM := $(word 3,$(subst ., ,$*)))
	echo "Building image for ${OS} ${ARCH} ${ARM}"
	$(DOCKER_CMD) build --platform $* \
		--build-arg CMD_NAME=$(CMDNAME) \
		$(DOCKER_LABELS) \
		$(IMAGE_TAGS) \
		--file ./Dockerfile $(OUTPUT_DIR)

.PHONY: docker-build
docker-build: docker/build/$(DEFAULT_DOCKER_PLATFORM)

.PHONY: docker-build-multiarch
docker-build-multiarch: build-multiarch
	@echo "Building image for following platforms: $(SUPPORTED_PLATFORMS)"
	$(DOCKER_CMD) buildx build --platform "$(DOCKER_SUPPORTED_PLATFORMS)" \
		--build-arg CMD_NAME=$(CMDNAME) \
		$(IMAGE_TAGS) \
		$(DOCKER_LABELS) \
		--file ./Dockerfile $(OUTPUT_DIR)
