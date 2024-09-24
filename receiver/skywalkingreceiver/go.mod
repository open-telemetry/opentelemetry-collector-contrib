module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver

go 1.22.0

require (
	github.com/gorilla/mux v1.8.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.110.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.110.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/skywalking v0.110.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.110.0
	go.opentelemetry.io/collector/component/componentstatus v0.110.0
	go.opentelemetry.io/collector/config/configgrpc v0.110.0
	go.opentelemetry.io/collector/config/confighttp v0.110.0
	go.opentelemetry.io/collector/config/confignet v1.16.0
	go.opentelemetry.io/collector/config/configtls v1.16.0
	go.opentelemetry.io/collector/confmap v1.16.0
	go.opentelemetry.io/collector/consumer v0.110.0
	go.opentelemetry.io/collector/consumer/consumertest v0.110.0
	go.opentelemetry.io/collector/pdata v1.16.0
	go.opentelemetry.io/collector/receiver v0.110.0
	go.opentelemetry.io/collector/semconv v0.110.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	google.golang.org/grpc v1.66.2
	google.golang.org/protobuf v1.34.2
	skywalking.apache.org/repo/goapi v0.0.0-20240104145220-ba7202308dd4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	go.opentelemetry.io/collector/client v1.16.0 // indirect
	go.opentelemetry.io/collector/component/componentprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.110.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.16.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.16.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.110.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.110.0 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/extension v0.110.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.110.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.16.0 // indirect
	go.opentelemetry.io/collector/internal/globalsignal v0.110.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.110.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.110.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverprofiles v0.110.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.55.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/skywalking => ../../pkg/translator/skywalking

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
