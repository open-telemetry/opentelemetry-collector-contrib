module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver

go 1.14

require (
	cloud.google.com/go v0.49.0 // indirect
	github.com/golang/protobuf v1.3.5
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gorilla/mux v1.7.4
	github.com/jaegertracing/jaeger v1.18.0
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/open-telemetry/opentelemetry-proto v0.3.0
	github.com/signalfx/sapm-proto v0.5.3
	github.com/smartystreets/assertions v0.0.0-20190215210624-980c5ac6f3ac // indirect
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.4.1-0.20200629224201-e7a7690e21fc
	go.uber.org/zap v1.13.0
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
