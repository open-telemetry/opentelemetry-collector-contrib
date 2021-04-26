BINARY_NAME ?= otelcol-sumo
VERSION ?= 0.25.0-sumo-1
BUILDER_VERSION ?= 0.25.0
BUILDER_BIN_PATH ?= ${HOME}/bin/opentelemetry-collector-builder

.PHONY: install
install:
	go install github.com/open-telemetry/opentelemetry-collector-builder@v$(BUILDER_VERSION)

.PHONY: install-prego1.16
install-prego1.16:
	GO111MODULE=on go get github.com/open-telemetry/opentelemetry-collector-builder@v$(BUILDER_VERSION)

.PHONY: _install-bin
_install-bin:
	mkdir -p ${HOME}/bin && \
	curl -L -o $(BUILDER_BIN_PATH) https://github.com/open-telemetry/opentelemetry-collector-builder/releases/download/v$(BUILDER_VERSION)/opentelemetry-collector-builder_$(BUILDER_VERSION)_$(PLATFORM)_amd64 && \
	chmod +x $(BUILDER_BIN_PATH)

.PHONY: install-bin-linux
install-bin-linux:
	$(MAKE) _install-bin PLATFORM=linux

.PHONY: install-bin-darwin
install-bin-darwin:
	$(MAKE) _install-bin PLATFORM=darwin

.PHONY: build
build:
	opentelemetry-collector-builder --config .otelcol-builder.yaml --name $(BINARY_NAME) --version $(VERSION)

.PHONY: otelcol-sumo-all-sys
otelcol-sumo-all-sys: otelcol-sumo-darwin_amd64 otelcol-sumo-linux_amd64 otelcol-sumo-linux_arm64 otelcol-sumo-windows_amd64

.PHONY: otelcol-sumo-darwin_amd64
otelcol-sumo-darwin_amd64:
	GOOS=darwin  GOARCH=amd64 $(MAKE) build BINARY_NAME=otelcol-sumo-darwin_amd64

.PHONY: otelcol-sumo-linux_amd64
otelcol-sumo-linux_amd64:
	GOOS=linux   GOARCH=amd64 $(MAKE) build BINARY_NAME=otelcol-sumo-linux_amd64

.PHONY: otelcol-sumo-linux_arm64
otelcol-sumo-linux_arm64:
	GOOS=linux   GOARCH=arm64 $(MAKE) build BINARY_NAME=otelcol-sumo-linux_arm64

.PHONY: otelcol-sumo-windows_amd64
otelcol-sumo-windows_amd64:
	GOOS=windows GOARCH=amd64 $(MAKE) build BINARY_NAME=otelcol-sumo-windows_amd64.exe
