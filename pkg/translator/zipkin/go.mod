module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin

go 1.17

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/google/go-cmp v0.5.7
	github.com/jaegertracing/jaeger v1.32.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.46.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.46.0
	github.com/openzipkin/zipkin-go v0.4.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector/model v0.46.0
	google.golang.org/protobuf v1.27.1

)

require (
	github.com/apache/thrift v0.16.0 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../../pkg/translator/opencensus
