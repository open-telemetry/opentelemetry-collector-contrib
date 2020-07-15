module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter

go 1.14

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	github.com/Microsoft/ApplicationInsights-Go v0.4.2
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/stretchr/testify v1.6.1
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	go.opentelemetry.io/collector v0.5.1-0.20200714180902-7066029f586b
	go.uber.org/zap v1.13.0
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	google.golang.org/appengine v1.6.2 // indirect
	google.golang.org/grpc v1.29.1
)
