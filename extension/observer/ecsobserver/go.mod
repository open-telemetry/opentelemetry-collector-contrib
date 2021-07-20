module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver

go 1.16

require (
	github.com/aws/aws-sdk-go v1.40.4
	github.com/hashicorp/golang-lru v0.5.4
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.30.2-0.20210719230137-809cae954ed3
	go.opentelemetry.io/collector/model v0.30.2-0.20210719230137-809cae954ed3 // indirect
	go.uber.org/multierr v1.7.0
	go.uber.org/zap v1.18.1
	gopkg.in/yaml.v2 v2.4.0
)
