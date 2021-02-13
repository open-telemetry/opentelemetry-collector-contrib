module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter

go 1.14

require (
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v0.0.2
	// The Loki repo doesn't currently follow the new go.mod semantics, since it's not typically used externally.
	// To get this to work as is, we must import it using the following format: v0.0.0-timestamp-commithash
	github.com/grafana/loki v0.0.0-20201223215703-1b79df3754f6
	github.com/prometheus/common v0.15.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.20.0
	go.uber.org/zap v1.16.0
)

// Keeping these the same as Loki (https://github.com/grafana/loki/blob/master/go.mod) to avoid dependency issues.
replace k8s.io/client-go => k8s.io/client-go v0.19.2
