module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver

go 1.14

require (
	contrib.go.opencensus.io/resource v0.1.2 // indirect
	github.com/bmizerany/perks v0.0.0-20141205001514-d9a9656a3a4b // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/prashantv/protectmem v0.0.0-20171002184600-e20412882b3a // indirect
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/streadway/quantile v0.0.0-20150917103942-b0c588724d25 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/uber/tchannel-go v1.16.0 // indirect
	go.opentelemetry.io/collector v0.5.1-0.20200708032135-c966e140fd4f
	go.uber.org/zap v1.13.0
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common
