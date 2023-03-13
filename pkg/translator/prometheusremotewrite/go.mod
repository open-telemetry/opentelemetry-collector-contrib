module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite

go 1.19

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.73.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.73.0
	github.com/prometheus/common v0.41.0
	github.com/prometheus/prometheus v0.42.1-0.20230210113933-af1d9e01c7e4
	github.com/stretchr/testify v1.8.2
	go.opentelemetry.io/collector/pdata v1.0.0-rc7
	go.opentelemetry.io/collector/semconv v0.73.0
	go.uber.org/multierr v1.10.0
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
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	go.opentelemetry.io/collector/featuregate v0.73.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	google.golang.org/genproto v0.0.0-20230124163310-31e0e69b6fc2 // indirect
	google.golang.org/grpc v1.53.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

retract v0.65.0
