module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor

go 1.21.0

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf/maps v0.1.1
	github.com/knadh/koanf/parsers/yaml v0.1.0
	github.com/knadh/koanf/providers/file v1.0.0
	github.com/knadh/koanf/providers/rawbytes v0.1.0
	github.com/knadh/koanf/v2 v2.1.1
	github.com/open-telemetry/opamp-go v0.15.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/config/configopaque v1.12.0
	go.opentelemetry.io/collector/config/configtls v1.12.0
	go.opentelemetry.io/collector/semconv v0.106.1
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.34.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.0.0-alpha.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
)
