module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/gobwas/glob v0.2.3
	github.com/gogo/protobuf v1.3.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.0.0-00010101000000-000000000000
	github.com/shirou/gopsutil v3.21.5+incompatible
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.2
	github.com/signalfx/signalfx-agent/pkg/apm v0.0.0-20201202163743-65b4fa925fc8
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210702000714-32c2d0f13167
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
	google.golang.org/protobuf v1.27.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ../../internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ../../pkg/batchperresourceattr

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../pkg/experimentalmetricmetadata

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210702000714-32c2d0f13167
