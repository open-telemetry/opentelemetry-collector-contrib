module github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsecshealthcheckextension

go 1.17

require (
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/knadh/koanf v1.2.3 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.35.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.35.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1
	golang.org/x/net v0.0.0-20210908191846-a5e095526f91 // indirect
	golang.org/x/sys v0.0.0-20210910150752-751e447fb3d0 // indirect
	google.golang.org/genproto v0.0.0-20210909211513-a8c4777a87af // indirect
	gotest.tools/v3 v3.0.3
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
