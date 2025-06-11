module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver

go 1.23.0

require (
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/service/xray v1.31.5
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy v0.128.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray v0.128.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.128.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.128.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.128.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/component/componenttest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/confignet v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/configtls v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap/xconfmap v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/consumer v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/consumer/consumertest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/pdata v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/pipeline v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/receiver v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/receiver/receiverhelper v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/receiver/receivertest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/otel v1.36.0
	go.opentelemetry.io/otel/sdk/metric v1.36.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/aws/aws-sdk-go v1.55.7 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.15 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.68 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.20 // indirect
	github.com/aws/smithy-go v1.22.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.128.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.128.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/featuregate v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ../../internal/aws/awsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy => ./../../internal/aws/proxy

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray => ./../../internal/aws/xray

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
