module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite

go 1.18

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.62.0
	github.com/prometheus/common v0.37.0
	github.com/prometheus/prometheus v0.38.0
	github.com/stretchr/testify v1.8.1
	go.opentelemetry.io/collector v0.62.2-0.20221025202712-36d142f003d8
	go.opentelemetry.io/collector/pdata v0.62.2-0.20221025202712-36d142f003d8
	go.opentelemetry.io/collector/semconv v0.62.2-0.20221025202712-36d142f003d8
	go.uber.org/multierr v1.8.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.0.0-20220809184613-07c6da5e1ced // indirect
	golang.org/x/sys v0.0.0-20220919091848-fb04ddd9f9c8 // indirect
	golang.org/x/text v0.4.0 // indirect
	google.golang.org/genproto v0.0.0-20220822174746-9e6da59bd2fc // indirect
	google.golang.org/grpc v1.50.1 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common
