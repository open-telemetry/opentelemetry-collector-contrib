include ./Makefile.Common

RUN_CONFIG=local/config.yaml

GIT_SHA=$(shell git rev-parse --short HEAD)
BUILD_INFO_IMPORT_PATH=github.com/open-telemetry/opentelemetry-collector-contrib/internal/version
BUILD_X1=-X $(BUILD_INFO_IMPORT_PATH).GitHash=$(GIT_SHA)
ifdef VERSION
BUILD_X2=-X $(BUILD_INFO_IMPORT_PATH).Version=$(VERSION)
endif
BUILD_INFO=-ldflags "${BUILD_X1} ${BUILD_X2}"

.DEFAULT_GOAL := all

.PHONY: all
all: common otelcontribcol

.PHONY: e2e-test
e2e-test: otelcontribcol
	$(MAKE) -C testbed runtests

.PHONY: ci
ci: all test-with-cover
	$(MAKE) -C testbed install-tools
	$(MAKE) -C testbed runtests

.PHONY: test-with-cover
test-with-cover:
	@echo Verifying that all packages have test files to count in coverage
	@scripts/check-test-files.sh $(subst github.com/open-telemetry/opentelemetry-collector-contrib/,./,$(ALL_PKGS))
	@echo pre-compiling tests
	set -e; for dir in $(ALL_TEST_DIRS); do \
	  echo "go test ./... + coverage in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST) $(GOTEST_OPT_WITH_COVERAGE) ./... && \
	 	go tool cover -html=coverage.txt -o coverage.html ); \
	done

.PHONY: install-tools
install-tools:
	GO111MODULE=on go install \
	  github.com/google/addlicense \
	  golang.org/x/lint/golint \
 	  github.com/golangci/golangci-lint/cmd/golangci-lint \
	  golang.org/x/tools/cmd/goimports \
	  github.com/client9/misspell/cmd/misspell \
	  honnef.co/go/tools/cmd/staticcheck \
	  github.com/pavius/impi/cmd/impi

.PHONY: otelcontribcol
otelcontribcol:
	GO111MODULE=on CGO_ENABLED=0 go build -o ./bin/$(GOOS)/otelcontribcol $(BUILD_INFO) ./cmd/otelcontribcol

.PHONY: run
run:
	GO111MODULE=on go run --race ./cmd/otelcontribcol/... --config ${RUN_CONFIG}

.PHONY: docker-component # Not intended to be used directly
docker-component: check-component
	GOOS=linux $(MAKE) $(COMPONENT)
	cp ./bin/linux/$(COMPONENT) ./cmd/$(COMPONENT)/
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

.PHONY: binaries
binaries: otelcontribcol

.PHONY: binaries-all-sys
binaries-all-sys:
	GOOS=darwin $(MAKE) binaries
	GOOS=linux $(MAKE) binaries
	GOOS=windows $(MAKE) binaries
