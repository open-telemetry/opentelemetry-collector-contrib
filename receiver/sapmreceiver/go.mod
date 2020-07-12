module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver

go 1.14

require (
	cloud.google.com/go v0.49.0 // indirect
	github.com/golang/protobuf v1.3.5
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gorilla/mux v1.7.4
	github.com/jaegertracing/jaeger v1.18.2-0.20200707061226-97d2319ff2be
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/open-telemetry/opentelemetry-proto v0.4.0
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/signalfx/sapm-proto v0.5.3
	github.com/smartystreets/assertions v0.0.0-20190215210624-980c5ac6f3ac // indirect
	github.com/stretchr/testify v1.6.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.5.1-0.20200712023338-3711c01b0c35
	go.uber.org/zap v1.13.0
	google.golang.org/appengine v1.6.5 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
