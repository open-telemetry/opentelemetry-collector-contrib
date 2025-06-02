module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver

go 1.23.0

require (
	github.com/google/go-cmp v0.7.0
	github.com/grobie/gomemcache v0.0.0-20230213081705-239240bbc445
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.127.0
	github.com/stretchr/testify v1.10.0
	github.com/testcontainers/testcontainers-go v0.37.0
	go.opentelemetry.io/collector/component v1.33.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/component/componenttest v0.127.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/config/confignet v1.33.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/confmap v1.33.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/consumer v1.33.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/consumer/consumertest v0.127.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/pdata v1.33.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/receiver v1.33.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/receiver/receivertest v0.127.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/scraper v0.127.1-0.20250528155941-4a3717978a51
	go.opentelemetry.io/collector/scraper/scraperhelper v0.127.1-0.20250528155941-4a3717978a51
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.0.1+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.127.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/shirou/gopsutil/v4 v4.25.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.127.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.127.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/collector/featuregate v1.33.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.127.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.127.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/collector/pipeline v0.127.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.127.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.127.1-0.20250528155941-4a3717978a51 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.19.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

// see https://github.com/distribution/distribution/issues/3590
exclude github.com/docker/distribution v2.8.0+incompatible

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
