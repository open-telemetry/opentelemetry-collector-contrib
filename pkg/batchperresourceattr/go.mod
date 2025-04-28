module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr

go 1.23.0

require (
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/consumer v1.30.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/consumer/consumertest v0.124.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/pdata v1.30.1-0.20250428165858-4ed72bda40bd
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
	go.opentelemetry.io/collector/consumer/xconsumer v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-20250428165858-4ed72bda40bd

replace go.opentelemetry.io/collector/config/configmiddleware v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/config/configmiddleware v0.0.0-20250428165858-4ed72bda40bd

replace go.opentelemetry.io/collector/extension/extensionmiddleware v1.30.0 => go.opentelemetry.io/collector/extension/extensionmiddleware v0.0.0-20250428165858-4ed72bda40bd
