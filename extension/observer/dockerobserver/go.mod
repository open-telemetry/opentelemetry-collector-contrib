module github.com/signalfx/forks/opentelemetry-collector-contrib/extension/observer/dockerobserver

go 1.16

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210827152330-09258f969908
	go.opentelemetry.io/collector/model v0.33.1-0.20210827152330-09258f969908 // indirect
	go.uber.org/zap v1.19.0
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.3 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../
