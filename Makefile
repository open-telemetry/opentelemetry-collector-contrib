include ./Makefile.Common

RUN_CONFIG=local/config.yaml

CMD?=
GIT_SHA=$(shell git rev-parse --short HEAD)
BUILD_INFO_IMPORT_PATH=github.com/open-telemetry/opentelemetry-collector-contrib/internal/version
BUILD_X1=-X $(BUILD_INFO_IMPORT_PATH).GitHash=$(GIT_SHA)
ifdef VERSION
BUILD_X2=-X $(BUILD_INFO_IMPORT_PATH).Version=$(VERSION)
endif
BUILD_X3=-X go.opentelemetry.io/collector/internal/version.BuildType=$(BUILD_TYPE)
BUILD_INFO=-ldflags "${BUILD_X1} ${BUILD_X2} ${BUILD_X3}"
STATIC_CHECK=staticcheck
OTEL_VERSION=master

# Modules to run integration tests on.
# XXX: Find a way to automatically populate this. Too slow to run across all modules when there are just a few.
INTEGRATION_TEST_MODULES := \
	extension/jmxmetricsextension/subprocess \
	receiver/dockerstatsreceiver \
	receiver/redisreceiver \
	internal/common

.DEFAULT_GOAL := all

.PHONY: all
all: common otelcontribcol

.PHONY: e2e-test
e2e-test: otelcontribcol
	$(MAKE) -C testbed run-tests

.PHONY: test-with-cover
unit-tests-with-cover:
	@echo Verifying that all packages have test files to count in coverage
	@internal/buildscripts/check-test-files.sh $(subst github.com/open-telemetry/opentelemetry-collector-contrib/,./,$(ALL_PKGS))
	@$(MAKE) for-all CMD="make do-unit-tests-with-cover"

.PHONY: integration-tests-with-cover
integration-tests-with-cover:
	@echo $(INTEGRATION_TEST_MODULES)
	@$(MAKE) for-all CMD="make do-integration-tests-with-cover" ALL_MODULES="$(INTEGRATION_TEST_MODULES)"

# Long-running e2e tests
.PHONY: stability-tests
stability-tests: otelcontribcol
	@echo Stability tests are disabled until we have a stable performance environment.
	@echo To enable the tests replace this echo by $(MAKE) -C testbed run-stability-tests

.PHONY: gotidy
gotidy:
	$(MAKE) for-all CMD="rm -fr go.sum"
	$(MAKE) for-all CMD="go mod tidy"

.PHONY: gofmt
gofmt:
	$(MAKE) for-all CMD="make fmt"

.PHONY: for-all
for-all:
	@echo "running $${CMD} in root"
	@$${CMD}
	@set -e; for dir in $(ALL_MODULES); do \
	  (cd "$${dir}" && \
	  	echo "running $${CMD} in $${dir}" && \
	 	$${CMD} ); \
	done

.PHONY: add-tag
add-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Adding tag ${TAG}"
	@git tag -a ${TAG} -s -m "Version ${TAG}"
	@set -e; for dir in $(ALL_MODULES); do \
	  (echo Adding tag "$${dir:2}/$${TAG}" && \
	 	git tag -a "$${dir:2}/$${TAG}" -s -m "Version ${dir:2}/${TAG}" ); \
	done

.PHONY: push-tag
push-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Pushing tag ${TAG}"
	@git push upstream ${TAG}
	@set -e; for dir in $(ALL_MODULES); do \
	  (echo Pushing tag "$${dir:2}/$${TAG}" && \
	 	git push upstream "$${dir:2}/$${TAG}"); \
	done

.PHONY: delete-tag
delete-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Deleting tag ${TAG}"
	@git tag -d ${TAG}
	@set -e; for dir in $(ALL_MODULES); do \
	  (echo Deleting tag "$${dir:2}/$${TAG}" && \
	 	git tag -d "$${dir:2}/$${TAG}" ); \
	done

GOMODULES = $(ALL_MODULES) $(PWD)
.PHONY: $(GOMODULES)
MODULEDIRS = $(GOMODULES:%=for-all-target-%)
for-all-target: $(MODULEDIRS)
$(MODULEDIRS):
	$(MAKE) -C $(@:for-all-target-%=%) $(TARGET)
.PHONY: for-all-target

.PHONY: install-tools
install-tools:
	go install github.com/client9/misspell/cmd/misspell
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go install github.com/google/addlicense
	go install github.com/jstemmer/go-junit-report
	go install github.com/pavius/impi/cmd/impi
	go install github.com/tcnksm/ghr
	go install honnef.co/go/tools/cmd/staticcheck
	go install go.opentelemetry.io/collector/cmd/issuegenerator

.PHONY: run
run:
	GO111MODULE=on go run --race ./cmd/otelcontribcol/... --config ${RUN_CONFIG}

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

.PHONY: otelcontribcol
otelcontribcol:
	GO111MODULE=on CGO_ENABLED=0 go build -o ./bin/otelcontribcol_$(GOOS)_$(GOARCH)$(EXTENSION) $(BUILD_INFO) ./cmd/otelcontribcol


.PHONY: otelcontribcol-all-sys
otelcontribcol-all-sys: otelcontribcol-darwin_amd64 otelcontribcol-linux_amd64 otelcontribcol-linux_arm64 otelcontribcol-windows_amd64

.PHONY: otelcontribcol-darwin_amd64
otelcontribcol-darwin_amd64:
	GOOS=darwin  GOARCH=amd64 $(MAKE) otelcontribcol

.PHONY: otelcontribcol-linux_amd64
otelcontribcol-linux_amd64:
	GOOS=linux   GOARCH=amd64 $(MAKE) otelcontribcol

.PHONY: otelcontribcol-linux_arm64
otelcontribcol-linux_arm64:
	GOOS=linux   GOARCH=arm64 $(MAKE) otelcontribcol

.PHONY: otelcontribcol-windows_amd64
otelcontribcol-windows_amd64:
	GOOS=windows GOARCH=amd64 EXTENSION=.exe $(MAKE) otelcontribcol

.PHONY: update-dep
update-dep:
	$(MAKE) for-all CMD="$(PWD)/internal/buildscripts/update-dep"
	$(MAKE) otelcontribcol
	$(MAKE) gotidy

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
	$(MAKE) for-all CMD="go mod edit -replace go.opentelemetry.io/collector=$(SRC_ROOT)/../opentelemetry-collector"

.PHONY: otel-from-lib
otel-from-lib:
	# Sets opentelemetry core to be not be pulled from local source tree. (Undoes otel-from-tree.)
	$(MAKE) for-all CMD="go mod edit -dropreplace go.opentelemetry.io/collector"

.PHONY: build-examples
build-examples:
	docker-compose -f examples/tracing/docker-compose.yml build
	docker-compose -f exporter/splunkhecexporter/example/docker-compose.yml build

.PHONY: deb-rpm-package
%-package: ARCH ?= amd64
%-package:
	$(MAKE) otelcontribcol-linux_$(ARCH)
	docker build -t otelcontribcol-fpm internal/buildscripts/packaging/fpm
	docker run --rm -v $(CURDIR):/repo -e PACKAGE=$* -e VERSION=$(VERSION) -e ARCH=$(ARCH) otelcontribcol-fpm