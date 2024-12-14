module github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension

go 1.22.0

require (
	github.com/stretchr/testify v1.10.0
	github.com/tg123/go-htpasswd v1.2.3
	go.opentelemetry.io/collector/client v1.21.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/component v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/component/componenttest v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/config/configopaque v1.21.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/confmap v1.21.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/extension v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/extension/auth v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/extension/extensiontest v0.115.1-0.20241206185113-3f3e208e71b8
	go.uber.org/goleak v1.3.0
	google.golang.org/grpc v1.68.1
)

require (
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/pdata v1.21.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/otel v1.32.0 // indirect
	go.opentelemetry.io/otel/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/sdk v1.32.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
