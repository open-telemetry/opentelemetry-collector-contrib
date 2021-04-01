module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.14

require (
	github.com/armon/go-metrics v0.3.3 // indirect
	github.com/aws/aws-sdk-go v1.38.11
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/gogo/googleapis v1.3.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.2.0
	github.com/hashicorp/go-immutable-radix v1.2.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws v0.0.0-00010101000000-000000000000
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.23.1-0.20210331235005-1edaf40cedaf
	go.uber.org/zap v1.16.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/ini.v1 v1.57.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws => ../../internal/aws
