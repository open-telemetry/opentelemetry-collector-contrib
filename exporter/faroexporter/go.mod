module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter

go 1.23.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/config/confighttp v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/config/configretry v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/config/configtls v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/consumer v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/exporter v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/pdata v1.27.1-0.20250307194215-7d3e03e500b0
	go.uber.org/zap v1.27.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal 