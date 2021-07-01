module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight

go 1.16

require (
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/atomic v1.8.0 // indirect
	go.uber.org/zap v1.18.1
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	golang.org/x/tools v0.1.3 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210630154722-7d0a0398174e
