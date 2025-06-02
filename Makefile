include ./Makefile.Common

RUN_CONFIG?=local/config.yaml
CMD?=
OTEL_VERSION=main
OTEL_STABLE_VERSION=main

VERSION=$(shell git describe --always --match "v[0-9]*" HEAD)
TRIMMED_VERSION=$(shell grep -o 'v[^-]*' <<< "$(VERSION)" | cut -c 2-)
CORE_VERSIONS=$(SRC_PARENT_DIR)/opentelemetry-collector/versions.yaml

COMP_REL_PATH=cmd/otelcontribcol/components.go
MOD_NAME=github.com/open-telemetry/opentelemetry-collector-contrib

GROUP ?= all
FOR_GROUP_TARGET=for-$(GROUP)-target

FIND_MOD_ARGS=-type f -name "go.mod"
TO_MOD_DIR=dirname {} \; | sort | grep -E '^./'
EX_COMPONENTS=-not -path "./receiver/*" -not -path "./processor/*" -not -path "./exporter/*" -not -path "./extension/*" -not -path "./connector/*"
EX_INTERNAL=-not -path "./internal/*"
EX_PKG=-not -path "./pkg/*"
EX_CMD=-not -path "./cmd/*"

# This includes a final slash
ROOT_DIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

RECEIVER_MODS_0 := $(shell find ./receiver/[a-f]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
RECEIVER_MODS_1 := $(shell find ./receiver/[g-o]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
RECEIVER_MODS_2 := $(shell find ./receiver/[p]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) ) # Prometheus is special and gets its own section.
RECEIVER_MODS_3 := $(shell find ./receiver/[q-z]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
RECEIVER_MODS := $(RECEIVER_MODS_0) $(RECEIVER_MODS_1) $(RECEIVER_MODS_2) $(RECEIVER_MODS_3)
PROCESSOR_MODS_0 := $(shell find ./processor/[a-o]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
PROCESSOR_MODS_1 := $(shell find ./processor/[p-z]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
PROCESSOR_MODS := $(PROCESSOR_MODS_0) $(PROCESSOR_MODS_1)
EXPORTER_MODS_0 := $(shell find ./exporter/[a-c]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
EXPORTER_MODS_1 := $(shell find ./exporter/[d-i]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
EXPORTER_MODS_2 := $(shell find ./exporter/[k-o]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
EXPORTER_MODS_3 := $(shell find ./exporter/[p-z]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
EXPORTER_MODS := $(EXPORTER_MODS_0) $(EXPORTER_MODS_1) $(EXPORTER_MODS_2) $(EXPORTER_MODS_3)
EXTENSION_MODS := $(shell find ./extension/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
CONNECTOR_MODS := $(shell find ./connector/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
INTERNAL_MODS := $(shell find ./internal/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
PKG_MODS := $(shell find ./pkg/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
CMD_MODS_0 := $(shell find ./cmd/[a-z]* $(FIND_MOD_ARGS) -not -path "./cmd/otel*col/*" -exec $(TO_MOD_DIR) )
CMD_MODS := $(CMD_MODS_0)
OTHER_MODS := $(shell find . $(EX_COMPONENTS) $(EX_INTERNAL) $(EX_PKG) $(EX_CMD) $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
ALL_MODS := $(RECEIVER_MODS) $(PROCESSOR_MODS) $(EXPORTER_MODS) $(EXTENSION_MODS) $(CONNECTOR_MODS) $(INTERNAL_MODS) $(PKG_MODS) $(CMD_MODS) $(OTHER_MODS)

CGO_MODS := ./receiver/hostmetricsreceiver

FIND_INTEGRATION_TEST_MODS={ find . -type f -name "*integration_test.go" & find . -type f -name "*e2e_test.go" -not -path "./testbed/*"; }
INTEGRATION_MODS := $(shell $(FIND_INTEGRATION_TEST_MODS) | xargs $(TO_MOD_DIR) | uniq)

# Excluded from ALL_MODS
GENERATED_MODS := $(shell find ./cmd/otel*col/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR))

ifeq ($(GOOS),windows)
	EXTENSION := .exe
endif

.DEFAULT_GOAL := all

all-modules:
	@echo $(ALL_MODS) | tr ' ' '\n' | sort

all-groups:
	@echo -e "receiver-0: $(RECEIVER_MODS_0)"
	@echo -e "\nreceiver-1: $(RECEIVER_MODS_1)"
	@echo -e "\nreceiver-2: $(RECEIVER_MODS_2)"
	@echo -e "\nreceiver-3: $(RECEIVER_MODS_3)"
	@echo -e "\nreceiver: $(RECEIVER_MODS)"
	@echo -e "\nprocessor-0: $(PROCESSOR_MODS_0)"
	@echo -e "\nprocessor-1: $(PROCESSOR_MODS_1)"
	@echo -e "\nprocessor: $(PROCESSOR_MODS)"
	@echo -e "\nexporter-0: $(EXPORTER_MODS_0)"
	@echo -e "\nexporter-1: $(EXPORTER_MODS_1)"
	@echo -e "\nexporter-2: $(EXPORTER_MODS_2)"
	@echo -e "\nexporter-3: $(EXPORTER_MODS_3)"
	@echo -e "\nexporter: $(EXPORTER_MODS)"
	@echo -e "\nextension: $(EXTENSION_MODS)"
	@echo -e "\nconnector: $(CONNECTOR_MODS)"
	@echo -e "\ninternal: $(INTERNAL_MODS)"
	@echo -e "\npkg: $(PKG_MODS)"
	@echo -e "\ncmd-0: $(CMD_MODS_0)"
	@echo -e "\ncmd: $(CMD_MODS)"
	@echo -e "\nother: $(OTHER_MODS)"
	@echo -e "\nintegration: $(INTEGRATION_MODS)"
	@echo -e "\ncgo: $(CGO_MODS)"
	@echo -e "\ngenerated: $(GENERATED_MODS)"

.PHONY: all
all: install-tools all-common goporto multimod-verify gotest otelcontribcol

.PHONY: all-common
all-common:
	@$(MAKE) $(FOR_GROUP_TARGET) TARGET="common"

.PHONY: e2e-test
e2e-test: otelcontribcol oteltestbedcol
	$(MAKE) --no-print-directory -C testbed run-tests

.PHONY: integration-test
integration-test:
	@$(MAKE) for-integration-target TARGET="mod-integration-test"

.PHONY: integration-tests-with-cover
integration-tests-with-cover:
	@$(MAKE) for-integration-target TARGET="do-integration-tests-with-cover"

# Long-running e2e tests
.PHONY: stability-tests
stability-tests: otelcontribcol
	@echo Stability tests are disabled until we have a stable performance environment.
	@echo To enable the tests replace this echo by $(MAKE) -C testbed run-stability-tests

.PHONY: genlabels
genlabels:
	@echo "Generating path-to-label mappings..."
	@echo "# This file is auto-generated. Do not edit manually." > .github/component_labels.txt
	@grep -E '^[A-Za-z0-9/]' .github/CODEOWNERS | \
		awk '{ print $$1 }' | \
		sed -E 's%(.+)/$$%\1%' | \
		while read -r COMPONENT; do \
			PREFIX=$$(printf '%s' "$${COMPONENT}" | sed -E 's%([^/])/.+%\1%'); \
			LABEL_NAME=$$(printf '%s\n' "$${COMPONENT}" | sed -E "s%^(.+)/(.+)$${PREFIX}%\1/\2%"); \
			if (( $${#LABEL_NAME} > 50 )); then \
				OIFS=$${IFS}; \
				IFS='/'; \
				for SEGMENT in $${COMPONENT}; do \
					r="/$${SEGMENT}\$$"; \
					if [[ "$${COMPONENT}" =~ $${r} ]]; then \
						break; \
					fi; \
					LABEL_NAME=$$(echo "$${LABEL_NAME}" | sed -E "s%^(.+)$${SEGMENT}\$$%\1%"); \
				done; \
				IFS=$${OIFS}; \
			fi; \
			echo "$${COMPONENT} $${LABEL_NAME}" >> .github/component_labels.txt; \
		done
	@echo "Labels generated and saved to .github/component_labels.txt"

.PHONY: gogci
gogci:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="gci"

.PHONY: tidylist
tidylist: $(CROSSLINK)
	cd internal/tidylist && \
	$(CROSSLINK) tidylist \
		--validate \
		--allow-circular allow-circular.txt \
		--skip cmd/otelcontribcol/go.mod \
		--skip cmd/oteltestbedcol/go.mod \
		tidylist.txt

# internal/tidylist/tidylist.txt lists modules in topological order, to ensure `go mod tidy` converges.
.PHONY: gotidy
gotidy:
	@for mod in $$(cat internal/tidylist/tidylist.txt); do \
		echo "Tidying $$mod"; \
		(cd $$mod && rm -rf go.sum && $(GOCMD) mod tidy -compat=1.23.0 && $(GOCMD) get toolchain@none) || exit $?; \
	done

.PHONY: remove-toolchain
remove-toolchain:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="toolchain"

.PHONY: gomoddownload
gomoddownload:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="moddownload"

.PHONY: gotest
gotest:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="test"

.PHONY: gotest-with-cover
gotest-with-cover:
	@$(MAKE) $(FOR_GROUP_TARGET) TARGET="test-with-cover"
	$(GOCMD) tool covdata textfmt -i=./coverage/unit -o ./$(GROUP)-coverage.txt

.PHONY: gotest-with-junit
gotest-with-junit:
	@$(MAKE) $(FOR_GROUP_TARGET) TARGET="test-with-junit"

.PHONY: gotest-with-junit-and-cover
gotest-with-junit-and-cover:
	@$(MAKE) $(FOR_GROUP_TARGET) TARGET="test-with-junit-and-cover"
	@go tool covdata textfmt -i=$(COVER_DIR_ABS) -o $(GROUP)-coverage.txt

.PHONY: gobuildtest
gobuildtest:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="buildtest"

.PHONY: gorunbuilttest
gorunbuilttest:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="runbuilttest"

.PHONY: gointegration-test
gointegration-test:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="mod-integration-test"

.PHONY: gointegration-sudo-test
gointegration-sudo-test:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="mod-integration-sudo-test"

.PHONY: gofmt
gofmt:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="fmt"

.PHONY: golint
golint:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="lint"

.PHONY: gogovulncheck
gogovulncheck:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="govulncheck"

.PHONY: goporto
goporto: $(PORTO)
	$(PORTO) -w --include-internal --skip-dirs "^cmd$$" ./

.PHONY: for-all
for-all:
	@set -e; for dir in $(ALL_MODS); do \
	  (cd "$${dir}" && \
	  	echo "running $${CMD} in $${dir}" && \
	 	$${CMD} ); \
	done

.PHONY: for-generated
for-generated:
	@set -e; for dir in $(GENERATED_MODS); do \
	  (cd "$${dir}" && \
	  	echo "running $${CMD} in $${dir}" && \
	 	$${CMD} ); \
	done

COMMIT?=HEAD
MODSET?=contrib-core
REMOTE?=git@github.com:open-telemetry/opentelemetry-collector-contrib.git
.PHONY: push-tags
push-tags: $(MULTIMOD)
	$(MULTIMOD) verify
	set -e; for tag in `$(MULTIMOD) tag -m ${MODSET} -c ${COMMIT} --print-tags | grep -v "Using" `; do \
		echo "pushing tag $${tag}"; \
		git push ${REMOTE} $${tag}; \
	done;

# Define a delegation target for each module
.PHONY: $(ALL_MODS)
$(ALL_MODS):
	@echo "Running target '$(TARGET)' in module '$@' as part of group '$(GROUP)'"
	$(MAKE) --no-print-directory -C $@ $(TARGET)

# Trigger each module's delegation target
.PHONY: for-all-target
for-all-target: $(ALL_MODS)

.PHONY: for-receiver-target
for-receiver-target: $(RECEIVER_MODS)

.PHONY: for-receiver-0-target
for-receiver-0-target: $(RECEIVER_MODS_0)

.PHONY: for-receiver-1-target
for-receiver-1-target: $(RECEIVER_MODS_1)

.PHONY: for-receiver-2-target
for-receiver-2-target: $(RECEIVER_MODS_2)

.PHONY: for-receiver-3-target
for-receiver-3-target: $(RECEIVER_MODS_3)

.PHONY: for-processor-target
for-processor-target: $(PROCESSOR_MODS)

.PHONY: for-processor-0-target
for-processor-0-target: $(PROCESSOR_MODS_0)

.PHONY: for-processor-1-target
for-processor-1-target: $(PROCESSOR_MODS_1)

.PHONY: for-exporter-target
for-exporter-target: $(EXPORTER_MODS)

.PHONY: for-exporter-0-target
for-exporter-0-target: $(EXPORTER_MODS_0)

.PHONY: for-exporter-1-target
for-exporter-1-target: $(EXPORTER_MODS_1)

.PHONY: for-exporter-2-target
for-exporter-2-target: $(EXPORTER_MODS_2)

.PHONY: for-exporter-3-target
for-exporter-3-target: $(EXPORTER_MODS_3)

.PHONY: for-extension-target
for-extension-target: $(EXTENSION_MODS)

.PHONY: for-connector-target
for-connector-target: $(CONNECTOR_MODS)

.PHONY: for-internal-target
for-internal-target: $(INTERNAL_MODS)

.PHONY: for-pkg-target
for-pkg-target: $(PKG_MODS)

.PHONY: for-cmd-target
for-cmd-target: $(CMD_MODS)

.PHONY: for-cmd-0-target
for-cmd-0-target: $(CMD_MODS_0)

.PHONY: for-other-target
for-other-target: $(OTHER_MODS)

.PHONY: for-integration-target
for-integration-target: $(INTEGRATION_MODS)

.PHONY: for-cgo-target
for-cgo-target: $(CGO_MODS)

# Debugging target, which helps to quickly determine whether for-all-target is working or not.
.PHONY: all-pwd
all-pwd:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="pwd"

.PHONY: run
run:
	cd ./cmd/otelcontribcol && GO111MODULE=on $(GOCMD) run --race . --config ../../${RUN_CONFIG} ${RUN_ARGS}

.PHONY: docker-component # Not intended to be used directly
docker-component: check-component
	GOOS=linux GOARCH=$(GOARCH) $(MAKE) $(COMPONENT)
	cp ./bin/$(COMPONENT)_linux_$(GOARCH) ./cmd/$(COMPONENT)/$(COMPONENT)
	docker build --platform linux/$(GOARCH) -t $(COMPONENT) ./cmd/$(COMPONENT)/
	rm ./cmd/$(COMPONENT)/$(COMPONENT)

.PHONY: check-component
check-component:
ifndef COMPONENT
	$(error COMPONENT variable was not defined)
endif

.PHONY: docker-otelcontribcol
docker-otelcontribcol:
	COMPONENT=otelcontribcol $(MAKE) docker-component

.PHONY: docker-supervisor-otelcontribcol
docker-supervisor-otelcontribcol: docker-otelcontribcol
	COMPONENT=opampsupervisor $(MAKE) docker-component

.PHONY: docker-telemetrygen
docker-telemetrygen:
	GOOS=linux GOARCH=$(GOARCH) $(MAKE) telemetrygen
	cp bin/telemetrygen_* cmd/telemetrygen/
	cd cmd/telemetrygen && docker build --platform linux/$(GOARCH) --build-arg="TARGETOS=$(GOOS)" --build-arg="TARGETARCH=$(GOARCH)" -t telemetrygen:latest .
	rm cmd/telemetrygen/telemetrygen_*

.PHONY: docker-golden
docker-golden:
	GOOS=linux GOARCH=$(GOARCH) $(MAKE) golden
	cp bin/golden_* cmd/golden/
	cd cmd/golden && docker build --platform linux/$(GOARCH) --build-arg="TARGETOS=$(GOOS)" --build-arg="TARGETARCH=$(GOARCH)" -t golden:latest .
	rm cmd/golden/golden_*

.PHONY: generate
generate: install-tools
	PATH="$(ROOT_DIR).tools:$$PATH" $(MAKE) for-all CMD="$(GOCMD) generate ./..."
	$(MAKE) gofmt

.PHONY: gengithub
gengithub: $(GITHUBGEN)
	$(GITHUBGEN)

.PHONY: gendistributions
gendistributions: $(GITHUBGEN)
	$(GITHUBGEN) distributions

gencodecov: $(CODECOVGEN)
	$(CODECOVGEN) --base-prefix github.com/open-telemetry/opentelemetry-collector-contrib --skipped-modules **/*test,**/examples/**,pkg/**,cmd/**,internal/**,*/encoding/**

.PHONY: update-codeowners
update-codeowners: generate gengithub
	$(MAKE) genlabels

.PHONY: gencodeowners
gencodeowners: install-tools
	$(GITHUBGEN) -skipgithub

FILENAME?=$(shell git branch --show-current)
.PHONY: chlog-new
chlog-new: $(CHLOGGEN)
	$(CHLOGGEN) new --config $(CHLOGGEN_CONFIG) --filename $(FILENAME)

.PHONY: chlog-validate
chlog-validate: $(CHLOGGEN)
	$(CHLOGGEN) validate --config $(CHLOGGEN_CONFIG)

.PHONY: chlog-preview
chlog-preview: $(CHLOGGEN)
	$(CHLOGGEN) update --config $(CHLOGGEN_CONFIG) --dry

.PHONY: chlog-update
chlog-update: $(CHLOGGEN)
	$(CHLOGGEN) update --config $(CHLOGGEN_CONFIG) --version $(VERSION)

.PHONY: genotelcontribcol
genotelcontribcol: $(BUILDER)
	./internal/buildscripts/ocb-add-replaces.sh otelcontribcol
	$(BUILDER) --skip-compilation --config cmd/otelcontribcol/builder-config-replaced.yaml

# Build the Collector executable.
.PHONY: otelcontribcol
otelcontribcol: genotelcontribcol
	cd ./cmd/otelcontribcol && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/otelcontribcol_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) .

# Build the Collector executable without the symbol table, debug information, and the DWARF symbol table.
.PHONY: otelcontribcollite
otelcontribcollite: genotelcontribcol
	cd ./cmd/otelcontribcol && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/otelcontribcol_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) -ldflags $(GO_BUILD_LDFLAGS) .

.PHONY: genoteltestbedcol
genoteltestbedcol: $(BUILDER)
	./internal/buildscripts/ocb-add-replaces.sh oteltestbedcol
	$(BUILDER) --skip-compilation --config cmd/oteltestbedcol/builder-config-replaced.yaml

# Build the Collector executable, with only components used in testbed.
.PHONY: oteltestbedcol
oteltestbedcol: genoteltestbedcol
	cd ./cmd/oteltestbedcol && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/oteltestbedcol_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) .

.PHONY: oteltestbedcollite
oteltestbedcollite: genoteltestbedcol
	cd ./cmd/oteltestbedcol && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/oteltestbedcol_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) -ldflags $(GO_BUILD_LDFLAGS) .

# Build the telemetrygen executable.
.PHONY: telemetrygen
telemetrygen:
	cd ./cmd/telemetrygen && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/telemetrygen_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) .

.PHONY: telemetrygenlite
telemetrygenlite:
	cd ./cmd/telemetrygen && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/telemetrygen_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) -ldflags $(GO_BUILD_LDFLAGS) .

# Build the Supervisor executable.
.PHONY: opampsupervisor
opampsupervisor:
	cd ./cmd/opampsupervisor && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/opampsupervisor_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) .

# Build the golden executable.
.PHONY: golden
golden:
	cd ./cmd/golden && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/golden_$(GOOS)_$(GOARCH)$(EXTENSION) \
		-tags $(GO_BUILD_TAGS) .

MODULES="internal/buildscripts/modules"
.PHONY: update-core-modules
update-core-module-list:
	BETA_LINE=$$(grep -n '  beta:' $(CORE_VERSIONS) | cut -d : -f 1); \
	(\
		echo -e '#!/bin/bash\n\nbeta_modules=('; \
		tail -n +$$BETA_LINE $(CORE_VERSIONS) | sed -En 's/^      - (.+)$$/  "\1"/p'; \
		echo -e ')\n\nstable_modules=('; \
		head -n $$BETA_LINE $(CORE_VERSIONS) | sed -En 's/^      - (.+)$$/  "\1"/p'; \
		echo -e ')' \
	) > $(MODULES);

# helper function to update the core packages in builder-config.yaml
# input parameters are
# $(1) = path/to/versions.yaml (where it greps the relevant packages)
# $(2) = path/to/go.mod (where it greps the package-versions)
# $(3) = path/to/builder-config.yaml (where we want to update the versions)
define updatehelper
	if [ ! -f $(1) ] || [ ! -f $(2) ] || [ ! -f $(3) ]; then \
			echo "Usage: updatehelper <versions.yaml> <go.mod> <builder-config.yaml>"; \
			exit 1; \
	fi
	grep "go\.opentelemetry\.io" $(1) | sed 's/^[[:space:]]*-[[:space:]]*//' | while IFS= read -r line; do \
			if grep -qF "$$line" $(2); then \
					package=$$(grep -F "$$line" $(2) | head -n 1 | awk '{print $$1}'); \
					version=$$(grep -F "$$line" $(2) | head -n 1 | awk '{print $$2}'); \
					builder_package=$$(grep -F "$$package" $(3) | awk '{print $$3}'); \
					builder_version=$$(grep -F "$$package" $(3) | awk '{print $$4}'); \
					if [ "$$builder_package" == "$$package" ]; then \
						sed -i.bak -e "s|$$builder_package.*$$builder_version|$$builder_package $$version|" $(3); \
						rm $(3).bak; \
						echo "[$(3)]: $$package updated from $$builder_version to $$version"; \
					fi; \
			fi; \
	done
endef


.PHONY: update-otel
update-otel:$(MULTIMOD)
	# Make sure cmd/otelcontribcol/go.mod and cmd/oteltestbedcol/go.mod are present
	$(MAKE) genotelcontribcol
	$(MAKE) genoteltestbedcol
	$(MULTIMOD) sync -s=true -o ../opentelemetry-collector -m stable --commit-hash "$(OTEL_STABLE_VERSION)"
	git add . && git commit -s -m "[chore] multimod update stable modules" ; \
	$(MULTIMOD) sync -s=true -o ../opentelemetry-collector -m beta --commit-hash "$(OTEL_VERSION)"
	git add . && git commit -s -m "[chore] multimod update beta modules" ; \
	$(MAKE) gotidy
	$(call updatehelper,$(CORE_VERSIONS),./cmd/otelcontribcol/go.mod,./cmd/otelcontribcol/builder-config.yaml)
	$(call updatehelper,$(CORE_VERSIONS),./cmd/oteltestbedcol/go.mod,./cmd/oteltestbedcol/builder-config.yaml)
	$(MAKE) -B install-tools
	$(MAKE) genotelcontribcol
	$(MAKE) genoteltestbedcol
	$(MAKE) generate
	$(MAKE) crosslink
	# Tidy again after generating code
	$(MAKE) gotidy
	$(MAKE) remove-toolchain
	git add . && git commit -s -m "[chore] mod and toolchain tidy" ; \

.PHONY: otel-from-tree
otel-from-tree:
	# This command allows you to make changes to your local checkout of otel core and build
	# contrib against those changes without having to push to github and update a bunch of
	# references. The workflow is:
	#
	# 1. Hack on changes in core (assumed to be checked out in ../opentelemetry-collector from this directory)
	# 2. Run `make otel-from-tree` (only need to run it once to remap go modules)
	# 3. You can now build contrib and it will use your local otel core changes.
	# 4. Before committing/pushing your contrib changes, undo by running `make otel-from-lib`.
	$(MAKE) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector=$(SRC_PARENT_DIR)/opentelemetry-collector"

.PHONY: otel-from-lib
otel-from-lib:
	# Sets opentelemetry core to be not be pulled from local source tree. (Undoes otel-from-tree.)
	$(MAKE) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector"

.PHONY: build-examples
build-examples:
	cd examples/secure-tracing/certs && $(MAKE) clean && $(MAKE) all && docker compose -f ../docker-compose.yaml build
	docker compose -f exporter/splunkhecexporter/example/docker-compose.yml build

.PHONY: deb-rpm-package
%-package: ARCH ?= amd64
%-package:
	GOOS=linux GOARCH=$(ARCH) $(MAKE) otelcontribcol
	docker build -t otelcontribcol-fpm internal/buildscripts/packaging/fpm
	docker run --rm -v $(CURDIR):/repo -e PACKAGE=$* -e VERSION=$(VERSION) -e ARCH=$(ARCH) otelcontribcol-fpm

# Verify existence of READMEs for components specified as default components in the collector.
.PHONY: checkdoc
checkdoc: $(CHECKFILE)
	$(CHECKFILE) --project-path $(CURDIR) --component-rel-path $(COMP_REL_PATH) --module-name $(MOD_NAME) --file-name "README.md"

# Verify existence of metadata.yaml for components specified as default components in the collector.
.PHONY: checkmetadata
checkmetadata: $(CHECKFILE)
	$(CHECKFILE) --project-path $(CURDIR) --component-rel-path $(COMP_REL_PATH) --module-name $(MOD_NAME) --file-name "metadata.yaml"

.PHONY: checkapi
checkapi: $(CHECKAPI)
	$(CHECKAPI) -folder . -config .checkapi.yaml

.PHONY: kind-ready
kind-ready:
	@if [ -n "$(shell kind get clusters -q)" ]; then echo "kind is ready"; else echo "kind not ready"; exit 1; fi

.PHONY: kind-build
kind-build: kind-ready docker-otelcontribcol
	docker tag otelcontribcol otelcontribcol-dev:0.0.1
	kind load docker-image otelcontribcol-dev:0.0.1

.PHONY: kind-install-daemonset
kind-install-daemonset: kind-ready kind-uninstall-daemonset## Install a local Collector version into the cluster.
	@echo "Installing daemonset collector"
	helm install daemonset-collector-dev open-telemetry/opentelemetry-collector --values ./examples/kubernetes/daemonset-collector-dev.yaml

.PHONY: kind-uninstall-daemonset
kind-uninstall-daemonset: kind-ready
	@echo "Uninstalling daemonset collector"
	helm uninstall --ignore-not-found daemonset-collector-dev

.PHONY: kind-install-deployment
kind-install-deployment: kind-ready kind-uninstall-deployment## Install a local Collector version into the cluster.
	@echo "Installing deployment collector"
	helm install deployment-collector-dev open-telemetry/opentelemetry-collector --values ./examples/kubernetes/deployment-collector-dev.yaml

.PHONY: kind-uninstall-deployment
kind-uninstall-deployment: kind-ready
	@echo "Uninstalling deployment collector"
	helm uninstall --ignore-not-found deployment-collector-dev

.PHONY: all-checklinks
all-checklinks:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="checklinks"

# Function to execute a command. Note the empty line before endef to make sure each command
# gets executed separately instead of concatenated with previous one.
# Accepts command to execute as first parameter.
define exec-command
$(1)

endef

# List of directories where certificates are stored for unit tests.
CERT_DIRS := receiver/sapmreceiver/testdata \
             receiver/signalfxreceiver/testdata \
             receiver/splunkhecreceiver/testdata \
             receiver/mongodbatlasreceiver/testdata/alerts/cert \
             receiver/mongodbreceiver/testdata/certs \
             receiver/cloudflarereceiver/testdata/cert

# Generate certificates for unit tests relying on certificates.
.PHONY: certs
certs:
	$(foreach dir, $(CERT_DIRS), $(call exec-command, @internal/buildscripts/gen-certs.sh -o $(dir)))

.PHONY: multimod-verify
multimod-verify: $(MULTIMOD)
	@echo "Validating versions.yaml"
	$(MULTIMOD) verify

.PHONY: multimod-prerelease
multimod-prerelease: $(MULTIMOD)
	$(MULTIMOD) prerelease -s=true -b=false -v ./versions.yaml -m contrib-base
	$(MAKE) gotidy

.PHONY: multimod-sync
multimod-sync: $(MULTIMOD)
	$(MULTIMOD) sync -a=true -s=true -o ../opentelemetry-collector
	$(MAKE) gotidy

.PHONY: crosslink
crosslink: $(CROSSLINK)
	@echo "Executing crosslink"
	$(CROSSLINK) --root=$(shell pwd) --prune

.PHONY: clean
clean:
	@echo "Removing coverage files"
	find . -type f -name 'coverage.txt' -delete
	find . -type f -name 'coverage.html' -delete
	find . -type f -name 'coverage.out' -delete
	find . -type f -name 'integration-coverage.txt' -delete
	find . -type f -name 'integration-coverage.html' -delete
	@echo "Removing built binary files"
	find . -type f -name 'builtunitetest.test' -delete

.PHONY: generate-gh-issue-templates
generate-gh-issue-templates:
	$(GITHUBGEN) issue-templates

.PHONY: checks
checks:
	$(MAKE) checkdoc
	$(MAKE) checkmetadata
	$(MAKE) checkapi
	$(MAKE) -j4 goporto
	$(MAKE) crosslink
	$(MAKE) -j4 gotidy
	$(MAKE) genotelcontribcol
	$(MAKE) genoteltestbedcol
	$(MAKE) gendistributions
	$(MAKE) -j4 generate
	$(MAKE) multimod-verify
	git diff --exit-code || (echo 'Some files need committing' && git status && exit 1)
