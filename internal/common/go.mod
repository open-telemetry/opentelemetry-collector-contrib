module github.com/open-telemetry/opentelemetry-collector-contrib/internal/common

go 1.19

require (
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230619144035-25129b794d48
	go.uber.org/zap v1.24.0
)

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
