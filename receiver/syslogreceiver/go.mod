module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver

go 1.14

require (
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-log-collection v0.15.2-0.20210303135848-fb7443a0caad
	github.com/orijtech/prometheus-go-metrics-exporter v0.0.6 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.21.1-0.20210305192649-7cdbd1caab8b
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.0+incompatible // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza => ../../internal/stanza
