module github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker

go 1.25.0

require (
	github.com/Microsoft/go-winio v0.6.2
	github.com/containerd/errdefs v1.0.0
	github.com/gobwas/glob v0.2.3
	github.com/moby/moby/api v1.54.2
	github.com/moby/moby/client v0.4.1
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/config/configoptional v1.56.1-0.20260423084629-98f555d14cd6
	go.opentelemetry.io/collector/config/configtls v1.56.1-0.20260423084629-98f555d14cd6
	go.opentelemetry.io/collector/confmap v1.56.1-0.20260423084629-98f555d14cd6
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.20.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.56.1-0.20260423084629-98f555d14cd6 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.150.1-0.20260423084629-98f555d14cd6 // indirect
	go.opentelemetry.io/collector/featuregate v1.56.1-0.20260423084629-98f555d14cd6 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
