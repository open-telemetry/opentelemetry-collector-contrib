module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.0.0-20200518175917-05cf2ea24e6c
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200712023338-3711c01b0c35
	go.uber.org/zap v1.13.0
	k8s.io/kubernetes v1.12.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
