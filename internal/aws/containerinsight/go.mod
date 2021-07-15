module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight

go 1.16

require (
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.30.0
	go.opentelemetry.io/collector/model v0.30.0
	go.uber.org/zap v1.18.1
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210712235908-f9dacb8402fe
