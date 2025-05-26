module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver

go 1.23.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.126.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.126.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.32.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/collector/confmap v1.32.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/collector/consumer v1.32.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/collector/receiver v1.32.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/goleak v1.3.0
	golang.org/x/sys v0.33.0
)

require (
	go.opentelemetry.io/collector/component/componenttest v0.126.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/collector/consumer/consumertest v0.126.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/collector/pdata v1.32.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/collector/pipeline v0.126.1-0.20250526141150-249e5d55ad2f
	go.opentelemetry.io/collector/receiver/receivertest v0.126.1-0.20250526141150-249e5d55ad2f
	go.uber.org/zap v1.27.0
)

require (
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/expr-lang/expr v1.17.3 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/leodido/go-syslog/v4 v4.2.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.126.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.126.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/extension v1.32.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/extension/xextension v0.126.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/featuregate v1.32.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.126.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.126.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.126.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.126.1-0.20250526141150-249e5d55ad2f // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.1 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
