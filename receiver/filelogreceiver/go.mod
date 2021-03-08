module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver

go 1.14

require (
	github.com/observiq/nanojack v0.0.0-20201106172433-343928847ebc
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-log-collection v0.15.2-0.20210303135848-fb7443a0caad
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.21.1-0.20210305192649-7cdbd1caab8b
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza => ../../internal/stanza
