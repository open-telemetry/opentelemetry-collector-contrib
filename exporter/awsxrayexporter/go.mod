module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.37.16
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.21.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray => ./../../internal/awsxray
