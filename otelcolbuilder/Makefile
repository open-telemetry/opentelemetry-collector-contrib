install:
	go install github.com/open-telemetry/opentelemetry-collector-builder@v0.24.0

install-prego1.16:
	GO111MODULE=on go get github.com/open-telemetry/opentelemetry-collector-builder@v0.24.0

build:
	opentelemetry-collector-builder --config .otelcol-builder.yaml
