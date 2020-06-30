module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.23.19
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/open-telemetry/opentelemetry-proto v0.3.0
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.4.1-0.20200630153623-992cdc57871e
	go.uber.org/zap v1.13.0
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
)
