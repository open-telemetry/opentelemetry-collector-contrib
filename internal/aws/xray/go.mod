module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray

go 1.23.0

require (
	github.com/aws/aws-sdk-go v1.55.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.121.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.27.1-0.20250313100724-0885401136ff
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.opentelemetry.io/collector/pdata v1.27.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ../../../internal/aws/awsutil

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
