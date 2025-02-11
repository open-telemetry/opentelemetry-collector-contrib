module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr

go 1.22.0

require (
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/consumer v1.24.1-0.20250123125445-24f88da7b583
	go.opentelemetry.io/collector/consumer/consumertest v0.118.1-0.20250123125445-24f88da7b583
	go.opentelemetry.io/collector/pdata v1.24.1-0.20250123125445-24f88da7b583
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.118.1-0.20250123125445-24f88da7b583 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.118.1-0.20250123125445-24f88da7b583 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/grpc v1.69.4 // indirect
	google.golang.org/protobuf v1.36.3 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
