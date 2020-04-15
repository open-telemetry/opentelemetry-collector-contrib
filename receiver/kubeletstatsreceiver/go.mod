module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200424155504-9d16f5971ef9
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.0.0-20200425013014-67eef23b1031
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
	k8s.io/kubernetes v1.12.0
)
