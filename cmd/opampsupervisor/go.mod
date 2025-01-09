module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor

go 1.22.0

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf/maps v0.1.1
	github.com/knadh/koanf/parsers/yaml v0.1.0
	github.com/knadh/koanf/providers/file v1.1.2
	github.com/knadh/koanf/providers/rawbytes v0.1.0
	github.com/knadh/koanf/v2 v2.1.2
	github.com/open-telemetry/opamp-go v0.17.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/config/configopaque v1.23.0
	go.opentelemetry.io/collector/config/configtls v1.23.0
	go.opentelemetry.io/collector/confmap v1.23.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.23.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.23.0
	go.opentelemetry.io/collector/semconv v0.117.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	golang.org/x/sys v0.29.0
	google.golang.org/protobuf v1.36.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
)
