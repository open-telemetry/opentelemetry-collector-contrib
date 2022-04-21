module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr

go 1.17

require (
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/collector v0.49.1-0.20220421154122-427f7dde5a8f
	go.opentelemetry.io/collector/pdata v0.49.1-0.20220421154122-427f7dde5a8f
	go.uber.org/multierr v1.8.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sys v0.0.0-20220128215802-99c3d69c2c27 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/grpc v1.45.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace go.opentelemetry.io/collector/semconv => go.opentelemetry.io/collector/semconv v0.0.0-20220421154122-427f7dde5a8f
