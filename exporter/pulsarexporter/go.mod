module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter

go 1.23.0

require (
	github.com/apache/pulsar-client-go v0.15.1
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/gogo/protobuf v1.3.2
	github.com/jaegertracing/jaeger-idl v0.5.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.128.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.128.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/component/componenttest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/configopaque v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/configretry v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap/xconfmap v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/consumer v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/consumer/consumererror v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/exporter v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/exporter/exportertest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/pdata v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/otel v1.36.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/99designs/keyring v1.2.1 // indirect
	github.com/AthenZ/athenz v1.12.13 // indirect
	github.com/DataDog/zstd v1.5.0 // indirect
	github.com/apache/thrift v0.21.0 // indirect
	github.com/ardielle/ardielle-go v1.5.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.4.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/danieljoos/wincred v1.1.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dvsekhvalnov/jose2go v1.6.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/hamba/avro/v2 v2.26.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.128.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/extension v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/featuregate v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pipeline v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.32.3 // indirect
	k8s.io/client-go v0.32.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20250321185631-1f6e0b77f77e // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../pkg/core/xidutils

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../pkg/translator/jaeger

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
