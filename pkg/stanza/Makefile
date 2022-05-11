GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

GIT_SHA=$(shell git rev-parse --short HEAD)

PROJECT_ROOT = $(shell pwd)
ALL_MODULES := $(shell find . -type f -name "go.mod" -exec dirname {} \; | sort )
ALL_SRC := $(shell find . -name '*.go' -type f | sort)
ADDLICENSE=addlicense
ALL_COVERAGE_MOD_DIRS := $(shell find . -type f -name 'go.mod' -exec dirname {} \; | egrep -v '^./internal/tools' | sort)
FIELDALIGNMENT_DIRS := ./pipeline/

TOOLS_MOD_DIR := ./internal/tools
.PHONY: install-tools
install-tools:
	cd $(TOOLS_MOD_DIR) && go install github.com/securego/gosec/v2/cmd/gosec@v2.9.6
	cd $(TOOLS_MOD_DIR) && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.43.0
	cd $(TOOLS_MOD_DIR) && go install github.com/vektra/mockery/cmd/mockery
	cd $(TOOLS_MOD_DIR) && go install github.com/google/addlicense
	cd $(TOOLS_MOD_DIR) && go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment

.PHONY: test
test: vet test-only

.PHONY: test-only
test-only:
	$(MAKE) for-all CMD="go test -race -coverprofile coverage.txt -coverpkg ./... ./..."

.PHONY: test-coverage
test-coverage: clean
	@set -e; \
	printf "" > coverage.txt; \
	for dir in $(ALL_COVERAGE_MOD_DIRS); do \
	  (cd "$${dir}" && \
	    go list ./... \
	    | grep -v third_party \
	    | xargs go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out && \
	  go tool cover -html=coverage.out -o coverage.html); \
	  [ -f "$${dir}/coverage.out" ] && cat "$${dir}/coverage.out" >> coverage.txt; \
	done; \
	sed -i.bak -e '2,$$ { /^mode: /d; }' coverage.txt	

.PHONY: bench
bench:
	go test -benchmem -run=^$$ -bench ^* ./...

.PHONY: clean
clean:
	$(MAKE) for-all CMD="rm -f coverage.txt.* coverage.html coverage.out"

.PHONY: tidy
tidy:
	$(MAKE) for-all CMD="rm -fr go.sum"
	$(MAKE) for-all CMD="go mod tidy -compat=1.17"

.PHONY: mod-update-only
mod-update-only:
	$(MAKE) for-all CMD="go get -u ./..."

.PHONY: mod-update
mod-update: mod-update-only tidy

.PHONY: listmod
listmod:
	@set -e; for dir in $(ALL_MODULES); do \
		(echo "$${dir}"); \
	done

.PHONY: lint
lint:
	golangci-lint run --allow-parallel-runners ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix --allow-parallel-runners ./...

.PHONY: fieldalignment
fieldalignment:
	fieldalignment $(FIELDALIGNMENT_DIRS)

.PHONY: fieldalignment-fix
fieldalignment-fix:
	fieldalignment -fix $(FIELDALIGNMENT_DIRS)

.PHONY: vet
vet:
	GOOS=darwin $(MAKE) for-all CMD="go vet ./..."
	GOOS=linux $(MAKE) for-all CMD="go vet ./..."
	GOOS=windows $(MAKE) for-all CMD="go vet ./..."

.PHONY: secure
secure:
	gosec ./...

.PHONY: generate
generate:
	go generate ./...

.PHONY: check-license
check-license:
	@ADDLICENSEOUT=`$(ADDLICENSE) -check $(ALL_SRC) 2>&1`; \
		if [ "$$ADDLICENSEOUT" ]; then \
			echo "$(ADDLICENSE) FAILED => add License errors:\n"; \
			echo "$$ADDLICENSEOUT\n"; \
			echo "Use 'make add-license' to fix this."; \
			exit 1; \
		else \
			echo "Check License finished successfully"; \
		fi

.PHONY: add-license
add-license:
	@ADDLICENSEOUT=`$(ADDLICENSE) -y "" -c "The OpenTelemetry Authors" $(ALL_SRC) 2>&1`; \
		if [ "$$ADDLICENSEOUT" ]; then \
			echo "$(ADDLICENSE) FAILED => add License errors:\n"; \
			echo "$$ADDLICENSEOUT\n"; \
			exit 1; \
		else \
			echo "Add License finished successfully"; \
		fi

.PHONY: for-all
for-all:
	@set -e; for dir in $(ALL_MODULES); do \
	  (cd "$${dir}" && $${CMD} ); \
	done

.PHONY: ci-check
ci-check: vet lint check-license secure fieldalignment
