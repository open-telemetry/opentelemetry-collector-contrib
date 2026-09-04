module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters

go 1.26.0

require (
	github.com/stretchr/testify v1.12.1
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
	golang.org/x/sys v0.47.0
)

require (
	go.uber.org/multierr v1.10.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
