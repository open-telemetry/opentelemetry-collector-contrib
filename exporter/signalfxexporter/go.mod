module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter

go 1.20

require (
	github.com/cenkalti/backoff/v4 v4.2.1
	github.com/gobwas/glob v0.2.3
	github.com/gogo/protobuf v1.3.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.85.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.85.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.85.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.85.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.85.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx v0.85.0
	github.com/shirou/gopsutil/v3 v3.23.8
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.3
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/component v0.85.1-0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/config/confighttp v0.85.1-0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/config/configopaque v0.85.1-0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/config/configtls v0.85.1-0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/confmap v0.85.1-0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/consumer v0.85.1-0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/exporter v0.85.1-0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0014.0.20230921012510-68dd7d763b59
	go.opentelemetry.io/collector/semconv v0.85.1-0.20230921012510-68dd7d763b59
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.26.0
	golang.org/x/sys v0.12.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.0 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.85.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rs/cors v1.10.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.85.1-0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/collector/config/configauth v0.85.1-0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/collector/config/configcompression v0.85.1-0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.85.1-0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/collector/config/internal v0.85.1-0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/collector/extension v0.85.1-0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/collector/extension/auth v0.85.1-0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0014.0.20230921012510-68dd7d763b59 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0 // indirect
	go.opentelemetry.io/otel v1.18.0 // indirect
	go.opentelemetry.io/otel/metric v1.18.0 // indirect
	go.opentelemetry.io/otel/sdk v1.18.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.41.0 // indirect
	go.opentelemetry.io/otel/trace v1.18.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
	google.golang.org/grpc v1.58.1 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ../../internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ../../pkg/batchperresourceattr

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../pkg/experimentalmetricmetadata

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx => ../../pkg/translator/signalfx

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

// https://github.com/go-openapi/spec/issues/156
replace github.com/go-openapi/spec v0.20.5 => github.com/go-openapi/spec v0.20.6
