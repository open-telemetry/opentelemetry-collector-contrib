module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin

go 1.18

require (
	github.com/census-instrumentation/opencensus-proto v0.4.1
	github.com/google/go-cmp v0.5.9
	github.com/jaegertracing/jaeger v1.38.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.60.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.60.0
	github.com/openzipkin/zipkin-go v0.4.0
	github.com/stretchr/testify v1.8.0
	go.opentelemetry.io/collector/pdata v0.60.1-0.20220923151520-96e9af35c002
	go.opentelemetry.io/collector/semconv v0.60.1-0.20220923151520-96e9af35c002
	google.golang.org/protobuf v1.28.1

)

require (
	github.com/apache/thrift v0.17.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.23.0 // indirect
	golang.org/x/net v0.0.0-20220624214902-1bab6f366d9e // indirect
	golang.org/x/sys v0.0.0-20220808155132-1c4a2a72c664 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220822174746-9e6da59bd2fc // indirect
	google.golang.org/grpc v1.49.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../../pkg/translator/opencensus
