module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter

go 1.24.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.19.1
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.0
	github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs v1.2.1
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.43.0
	go.opentelemetry.io/collector/component/componenttest v0.137.0
	go.opentelemetry.io/collector/confmap v1.43.0
	go.opentelemetry.io/collector/consumer v1.43.0
	go.opentelemetry.io/collector/exporter v1.43.0
	go.opentelemetry.io/collector/exporter/exporterhelper v0.137.0
	go.opentelemetry.io/collector/exporter/exportertest v0.137.0
	go.opentelemetry.io/collector/pdata v1.43.0
	go.opentelemetry.io/collector/pipeline v1.43.0
	go.uber.org/zap v1.27.0
)
