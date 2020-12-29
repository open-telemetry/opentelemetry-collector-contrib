module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver

go 1.14

require (
	github.com/aws/aws-sdk-go v1.36.17
	github.com/hashicorp/golang-lru v0.5.4
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.17.0
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../
