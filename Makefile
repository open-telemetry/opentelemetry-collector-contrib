include ./Makefile.Common

RUN_CONFIG?=local/config.yaml
CMD?=
OTEL_VERSION=main

BUILD_INFO_IMPORT_PATH=github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelcontribcore/internal/version
VERSION=$(shell git describe --always --match "v[0-9]*" HEAD)
BUILD_INFO=-ldflags "-X $(BUILD_INFO_IMPORT_PATH).Version=$(VERSION)"

COMP_REL_PATH=internal/components/components.go
MOD_NAME=github.com/open-telemetry/opentelemetry-collector-contrib

GROUP ?= all
FOR_GROUP_TARGET=for-$(GROUP)-target

FIND_MOD_ARGS=-type f -name "go.mod"
TO_MOD_DIR=dirname {} \; | sort | egrep  '^./'
EX_COMPONENTS=-not -path "./receiver/*" -not -path "./processor/*" -not -path "./exporter/*" -not -path "./extension/*"
EX_INTERNAL=-not -path "./internal/*"

# NONROOT_MODS includes ./* dirs (excludes . dir)
NONROOT_MODS := $(shell find . $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )

RECEIVER_MODS_0 := $(shell find ./receiver/[a-k]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
RECEIVER_MODS_1 := $(shell find ./receiver/[l-z]* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
RECEIVER_MODS := $(RECEIVER_MODS_0) $(RECEIVER_MODS_1)
PROCESSOR_MODS := $(shell find ./processor/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
EXPORTER_MODS := $(shell find ./exporter/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
EXTENSION_MODS := $(shell find ./extension/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
INTERNAL_MODS := $(shell find ./internal/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
OTHER_MODS := $(shell find . $(EX_COMPONENTS) $(EX_INTERNAL) $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) ) $(PWD)
ALL_MODS := $(RECEIVER_MODS) $(PROCESSOR_MODS) $(EXPORTER_MODS) $(EXTENSION_MODS) $(INTERNAL_MODS) $(OTHER_MODS)

# find -exec dirname cannot be used to process multiple matching patterns
FIND_INTEGRATION_TEST_MODS={ find . -type f -name "*integration_test.go" & find . -type f -name "*e2e_test.go" -not -path "./testbed/*"; }
INTEGRATION_MODS := $(shell $(FIND_INTEGRATION_TEST_MODS) | uniq | xargs $(TO_MOD_DIR) )

ifeq ($(GOOS),windows)
	EXTENSION := .exe
endif

.DEFAULT_GOAL := all

all-modules:
	@echo $(NONROOT_MODS) | tr ' ' '\n' | sort

all-groups:
	@echo "receiver-0: $(RECEIVER_MODS_0)"
	@echo "\nreceiver-1: $(RECEIVER_MODS_1)"
	@echo "\nreceiver: $(RECEIVER_MODS)"
	@echo "\nprocessor: $(PROCESSOR_MODS)"
	@echo "\nexporter: $(EXPORTER_MODS)"
	@echo "\nextension: $(EXTENSION_MODS)"
	@echo "\ninternal: $(INTERNAL_MODS)"
	@echo "\nother: $(OTHER_MODS)"

.PHONY: all
all: install-tools all-common gotest otelcontribcol otelcontribcol-unstable

.PHONY: all-common
all-common:
	@$(MAKE) $(FOR_GROUP_TARGET) TARGET="common"

.PHONY: e2e-test
e2e-test: otelcontribcol otelcontribcol-unstable otelcontribcol-testbed
	$(MAKE) -C testbed run-tests

.PHONY: unit-tests-with-cover
unit-tests-with-cover:
	@echo Verifying that all packages have test files to count in coverage
	@internal/buildscripts/check-test-files.sh $(subst github.com/open-telemetry/opentelemetry-collector-contrib/,./,$(ALL_PKGS))
	@$(MAKE) $(FOR_GROUP_TARGET) TARGET="do-unit-tests-with-cover"

TARGET="do-integration-tests-with-cover"
.PHONY: integration-tests-with-cover
integration-tests-with-cover: $(INTEGRATION_MODS)

# Long-running e2e tests
.PHONY: stability-tests
stability-tests: otelcontribcol
	@echo Stability tests are disabled until we have a stable performance environment.
	@echo To enable the tests replace this echo by $(MAKE) -C testbed run-stability-tests

.PHONY: gotidy
gotidy:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="tidy"

.PHONY: gomoddownload
gomoddownload:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="moddownload"

.PHONY: gotest
gotest:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="test"

.PHONY: gofmt
gofmt:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="fmt"

.PHONY: golint
golint:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="lint"

.PHONY: goimpi
goimpi:
	@$(MAKE) $(FOR_GROUP_TARGET) TARGET="impi"

.PHONY: goporto
goporto:
	porto -w --include-internal --skip-dirs "^cmd$$" ./

.PHONY: for-all
for-all:
	@echo "running $${CMD} in root"
	@$${CMD}
	@set -e; for dir in $(NONROOT_MODS); do \
	  (cd "$${dir}" && \
	  	echo "running $${CMD} in $${dir}" && \
	 	$${CMD} ); \
	done

.PHONY: add-tag
add-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Adding tag ${TAG}"
	@git tag -a ${TAG} -s -m "Version ${TAG}"
	@set -e; for dir in $(NONROOT_MODS); do \
	  (echo Adding tag "$${dir:2}/$${TAG}" && \
	 	git tag -a "$${dir:2}/$${TAG}" -s -m "Version ${dir:2}/${TAG}" ); \
	done

.PHONY: push-tag
push-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Pushing tag ${TAG}"
	@git push git@github.com:open-telemetry/opentelemetry-collector-contrib.git ${TAG}
	@set -e; for dir in $(NONROOT_MODS); do \
	  (echo Pushing tag "$${dir:2}/$${TAG}" && \
	 	git push git@github.com:open-telemetry/opentelemetry-collector-contrib.git "$${dir:2}/$${TAG}"); \
	done

.PHONY: delete-tag
delete-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Deleting tag ${TAG}"
	@git tag -d ${TAG}
	@set -e; for dir in $(NONROOT_MODS); do \
	  (echo Deleting tag "$${dir:2}/$${TAG}" && \
	 	git tag -d "$${dir:2}/$${TAG}" ); \
	done

DEPENDABOT_PATH=".github/dependabot.yml"
.PHONY: gendependabot
gendependabot:
	@echo "Recreating ${DEPENDABOT_PATH} file"
	@echo "# File generated by \"make gendependabot\"; DO NOT EDIT." > ${DEPENDABOT_PATH}
	@echo "" >> ${DEPENDABOT_PATH}
	@echo "version: 2" >> ${DEPENDABOT_PATH}
	@echo "updates:" >> ${DEPENDABOT_PATH}
	@echo "Add entry for \"/\" github-actions"
	@echo "  - package-ecosystem: \"github-actions\"" >> ${DEPENDABOT_PATH}
	@echo "    directory: \"/\"" >> ${DEPENDABOT_PATH}
	@echo "    schedule:" >> ${DEPENDABOT_PATH}
	@echo "      interval: \"weekly\"" >> ${DEPENDABOT_PATH}
	@echo "Add entry for \"/\" docker"
	@echo "  - package-ecosystem: \"docker\"" >> ${DEPENDABOT_PATH}
	@echo "    directory: \"/\"" >> ${DEPENDABOT_PATH}
	@echo "    schedule:" >> ${DEPENDABOT_PATH}
	@echo "      interval: \"weekly\"" >> ${DEPENDABOT_PATH}
	@echo "Add entry for \"/\" gomod"
	@echo "  - package-ecosystem: \"gomod\"" >> ${DEPENDABOT_PATH}
	@echo "    directory: \"/\"" >> ${DEPENDABOT_PATH}
	@echo "    schedule:" >> ${DEPENDABOT_PATH}
	@echo "      interval: \"weekly\"" >> ${DEPENDABOT_PATH}
	@set -e; for dir in $(NONROOT_MODS); do \
		echo "Add entry for \"$${dir:1}\""; \
		echo "  - package-ecosystem: \"gomod\"" >> ${DEPENDABOT_PATH}; \
		echo "    directory: \"$${dir:1}\"" >> ${DEPENDABOT_PATH}; \
		echo "    schedule:" >> ${DEPENDABOT_PATH}; \
		echo "      interval: \"weekly\"" >> ${DEPENDABOT_PATH}; \
	done

# Define a delegation target for each module
.PHONY: $(ALL_MODS)
$(ALL_MODS):
	@echo "Running target '$(TARGET)' in module '$@' as part of group '$(GROUP)'"
	$(MAKE) -C $@ $(TARGET)

# Trigger each module's delegation target
.PHONY: for-all-target
for-all-target: $(ALL_MODS)

.PHONY: for-receiver-target
for-receiver-target: $(RECEIVER_MODS)

.PHONY: for-receiver-0-target
for-receiver-0-target: $(RECEIVER_MODS_0)

.PHONY: for-receiver-1-target
for-receiver-1-target: $(RECEIVER_MODS_1)

.PHONY: for-processor-target
for-processor-target: $(PROCESSOR_MODS)

.PHONY: for-exporter-target
for-exporter-target: $(EXPORTER_MODS)

.PHONY: for-extension-target
for-extension-target: $(EXTENSION_MODS)

.PHONY: for-internal-target
for-internal-target: $(INTERNAL_MODS)

.PHONY: for-other-target
for-other-target: $(OTHER_MODS)

# Debugging target, which helps to quickly determine whether for-all-target is working or not.
.PHONY: all-pwd
all-pwd:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="pwd"

TOOLS_MOD_DIR := ./internal/tools
.PHONY: install-tools
install-tools:
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/client9/misspell/cmd/misspell
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/google/addlicense
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/jstemmer/go-junit-report
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/pavius/impi/cmd/impi
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/tcnksm/ghr
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/checkdoc
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/issuegenerator
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install golang.org/x/tools/cmd/goimports
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/multimod
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/jcchavezs/porto/cmd/porto
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/crosslink

.PHONY: run
run:
	GO111MODULE=on $(GOCMD) run --race ./cmd/otelcontribcol/... --config ${RUN_CONFIG} ${RUN_ARGS}

.PHONY: docker-component # Not intended to be used directly
docker-component: check-component
	GOOS=linux GOARCH=amd64 $(MAKE) $(COMPONENT)
	cp ./bin/$(COMPONENT)_linux_amd64 ./cmd/$(COMPONENT)/$(COMPONENT)
	docker build -t $(COMPONENT) ./cmd/$(COMPONENT)/
	rm ./cmd/$(COMPONENT)/$(COMPONENT)

.PHONY: check-component
check-component:
ifndef COMPONENT
	$(error COMPONENT variable was not defined)
endif

.PHONY: docker-otelcontribcol
docker-otelcontribcol:
	COMPONENT=otelcontribcol $(MAKE) docker-component

.PHONY: generate
generate:
	cd cmd/mdatagen && $(GOCMD) install .
	$(MAKE) for-all CMD="$(GOCMD) generate ./..."

.PHONY: chlog-install
chlog-install:
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/chloggen

FILENAME?=$(shell git branch --show-current)
.PHONY: chlog-new
chlog-new: chlog-install
	chloggen new --filename $(FILENAME)

.PHONY: chlog-validate
chlog-validate: chlog-install
	chloggen validate

.PHONY: chlog-preview
chlog-preview: chlog-install
	chloggen update --dry

.PHONY: chlog-update
chlog-update: chlog-install
	chloggen update --version $(VERSION)

# Build the Collector executable.
.PHONY: otelcontribcol
otelcontribcol:
	GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ./bin/otelcontribcol_$(GOOS)_$(GOARCH)$(EXTENSION) \
		$(BUILD_INFO) -tags $(GO_BUILD_TAGS) ./cmd/otelcontribcol

# Build the Collector executable, including unstable functionality.
.PHONY: otelcontribcol-unstable
otelcontribcol-unstable:
	GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ./bin/otelcontribcol_unstable_$(GOOS)_$(GOARCH)$(EXTENSION) \
		$(BUILD_INFO) -tags $(GO_BUILD_TAGS),enable_unstable ./cmd/otelcontribcol

# Build the Collector executable, with only components used in testbed.
.PHONY: otelcontribcol-testbed
otelcontribcol-testbed:
	GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ./bin/otelcontribcol_testbed_$(GOOS)_$(GOARCH)$(EXTENSION) \
		$(BUILD_INFO) -tags $(GO_BUILD_TAGS),testbed ./cmd/otelcontribcol

.PHONY: update-dep
update-dep:
	$(MAKE) $(FOR_GROUP_TARGET) TARGET="updatedep"
	$(MAKE) otelcontribcol

.PHONY: update-otel
update-otel:
	$(MAKE) update-dep MODULE=go.opentelemetry.io/collector VERSION=$(OTEL_VERSION)

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
	$(MAKE) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector=$(SRC_ROOT)/../opentelemetry-collector"

.PHONY: otel-from-lib
otel-from-lib:
	# Sets opentelemetry core to be not be pulled from local source tree. (Undoes otel-from-tree.)
	$(MAKE) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector"

.PHONY: build-examples
build-examples:
	docker-compose -f examples/tracing/docker-compose.yml build
	docker-compose -f exporter/splunkhecexporter/example/docker-compose.yml build

.PHONY: deb-rpm-package
%-package: ARCH ?= amd64
%-package:
	GOOS=linux GOARCH=$(ARCH) $(MAKE) otelcontribcol
	docker build -t otelcontribcol-fpm internal/buildscripts/packaging/fpm
	docker run --rm -v $(CURDIR):/repo -e PACKAGE=$* -e VERSION=$(VERSION) -e ARCH=$(ARCH) otelcontribcol-fpm

# Verify existence of READMEs for components specified as default components in the collector.
.PHONY: checkdoc
checkdoc:
	checkdoc --project-path $(CURDIR) --component-rel-path $(COMP_REL_PATH) --module-name $(MOD_NAME)

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
             receiver/mongodbreceiver/testdata/certs

# Generate certificates for unit tests relying on certificates.
.PHONY: certs
certs:
	$(foreach dir, $(CERT_DIRS), $(call exec-command, @internal/buildscripts/gen-certs.sh -o $(dir)))

.PHONY: multimod-verify
multimod-verify: install-tools
	@echo "Validating versions.yaml"
	multimod verify

.PHONY: multimod-prerelease
multimod-prerelease: install-tools
	multimod prerelease -s=true -b=false -v ./versions.yaml -m contrib-base
	$(MAKE) gotidy

.PHONY: crosslink
crosslink: install-tools
	@echo "Executing crosslink"
	crosslink --root=$(shell pwd)

.PHONY: clean
clean:
	@echo "Removing coverage files"
	find . -type f -name 'coverage.txt' -delete
	find . -type f -name 'coverage.html' -delete
	find . -type f -name 'integration-coverage.txt' -delete
	find . -type f -name 'integration-coverage.html' -delete

.PHONY: genconfigdocs
genconfigdocs:
	cd cmd/configschema && $(GOCMD) run ./docsgen all

.PHONY: generate-all-labels
generate-all-labels:
	$(MAKE) generate-labels TYPE="cmd" COLOR="#483C32"
	$(MAKE) generate-labels TYPE="pkg" COLOR="#F9DE22"
	$(MAKE) generate-labels TYPE="extension" COLOR="#FF794D"
	$(MAKE) generate-labels TYPE="receiver" COLOR="#E91B7B"
	$(MAKE) generate-labels TYPE="processor" COLOR="#800080"
	$(MAKE) generate-labels TYPE="exporter" COLOR="#50C878"

.PHONY: generate-labels
generate-labels:
	if [ -z $${TYPE+x} ] || [ -z $${COLOR+x} ]; then \
		echo "Must provide a TYPE and COLOR"; \
		exit 1; \
	fi; \
	echo "Generating labels for $${TYPE}" ; \
	COMPONENTS=$$(find ./$${TYPE} -type d -maxdepth 1 -mindepth 1 -exec basename \{\} \;); \
	for comp in $${COMPONENTS}; do \
		NAME=$${comp//"$${TYPE}"}; \
		gh label create "$${TYPE}/$${NAME}" -c "$${COLOR}"; \
	done; \
	exit 0
