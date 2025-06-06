module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter

go 1.23.0

require (
	github.com/census-instrumentation/opencensus-proto v0.4.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.127.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.33.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/component/componenttest v0.127.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/config/configgrpc v0.127.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/config/configopaque v1.33.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/config/configretry v1.33.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/config/configtls v1.33.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/confmap v1.33.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/confmap/xconfmap v0.127.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/consumer v1.33.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/consumer/consumertest v0.127.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/exporter v0.127.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/exporter/exportertest v0.127.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/pdata v1.33.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/pdata/testdata v0.127.1-0.20250605211604-c9aaed834963
	go.opentelemetry.io/collector/receiver/receivertest v0.127.1-0.20250605211604-c9aaed834963
	go.uber.org/goleak v1.3.0
	google.golang.org/grpc v1.72.2
)

require (
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.127.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.127.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.33.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/config/configauth v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.33.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/config/confignet v1.33.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/extension v1.33.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.33.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/featuregate v1.33.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/pipeline v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/receiver v1.33.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.127.1-0.20250605211604-c9aaed834963 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.61.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250303144028-a0af3efb3deb // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250519155744-55703ea1f237 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver => ../../receiver/opencensusreceiver

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
