module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter

go 1.14

require (
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v0.0.2
	github.com/grafana/loki v1.6.1
	github.com/prometheus/common v0.15.0
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.18.0
	go.uber.org/zap v1.16.0
)

// Keeping this same as Loki (https://github.com/grafana/loki/blob/master/go.mod) to avoid dependency issues.
replace k8s.io/client-go => k8s.io/client-go v0.19.2
