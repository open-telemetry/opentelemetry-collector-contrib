module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver

go 1.14

require (
	cloud.google.com/go v0.49.0 // indirect
	contrib.go.opencensus.io/resource v0.1.2 // indirect
	github.com/bmizerany/perks v0.0.0-20141205001514-d9a9656a3a4b // indirect
	github.com/golang/protobuf v1.3.5
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gorilla/mux v1.7.4
	github.com/jaegertracing/jaeger v1.18.2-0.20200707061226-97d2319ff2be
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/open-telemetry/opentelemetry-proto v0.3.0
	github.com/prashantv/protectmem v0.0.0-20171002184600-e20412882b3a // indirect
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/signalfx/sapm-proto v0.5.3
	github.com/smartystreets/assertions v0.0.0-20190215210624-980c5ac6f3ac // indirect
	github.com/streadway/quantile v0.0.0-20150917103942-b0c588724d25 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/uber/tchannel-go v1.16.0 // indirect
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.5.1-0.20200708032135-c966e140fd4f
	go.uber.org/zap v1.13.0
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
