module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0
	github.com/shirou/gopsutil v0.0.0-20200517204708-c89193f22d93
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200723232356-d4053cc823a0
	go.uber.org/zap v1.15.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../
