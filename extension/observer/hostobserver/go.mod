module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0-00010101000000-000000000000
	github.com/shirou/gopsutil v2.20.9+incompatible
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.12.1-0.20201016230751-46aada6e3c3a
	go.uber.org/zap v1.16.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../
