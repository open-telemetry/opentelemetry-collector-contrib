module github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension

go 1.19

require (
	github.com/stretchr/testify v1.8.3
	go.opentelemetry.io/collector v0.78.3-0.20230525165144-87dd85a6c034
	go.opentelemetry.io/collector/component v0.78.3-0.20230525165144-87dd85a6c034
	go.opentelemetry.io/collector/confmap v0.78.3-0.20230527001353-14c039d701ce
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.24.0
	golang.org/x/oauth2 v0.8.0
	google.golang.org/grpc v1.55.0
)

require (
	cloud.google.com/go/compute v1.19.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.9.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230525165144-87dd85a6c034 // indirect
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012.0.20230525165144-87dd85a6c034 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.42.0 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
