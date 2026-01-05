module github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka

go 1.24.0

require (
	github.com/IBM/sarama v1.46.3
	github.com/aws/aws-msk-iam-sasl-signer-go v1.0.4
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka v0.142.0
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9
	github.com/stretchr/testify v1.11.1
	github.com/twmb/franz-go/pkg/kfake v0.0.0-20251021233722-4ca18825d8c0
	github.com/twmb/franz-go/pkg/sasl/kerberos v1.1.0
	github.com/xdg-go/scram v1.2.0
	go.opentelemetry.io/collector/config/configcompression v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/configopaque v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/configtls v1.48.1-0.20251231114627-ad49dc64648d
	go.uber.org/goleak v1.3.0
)

require go.opentelemetry.io/collector/confmap/xconfmap v0.142.1-0.20251231114627-ad49dc64648d // indirect

require (
	github.com/aws/aws-sdk-go-v2 v1.36.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.16 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.69 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.31 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.35 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.35 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.21 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/twmb/franz-go v1.20.6
	github.com/twmb/franz-go/pkg/kadm v1.17.1
	github.com/twmb/franz-go/pkg/kmsg v1.12.0
	github.com/twmb/franz-go/plugin/kzap v1.1.2
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	go.opentelemetry.io/collector/confmap v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/featuregate v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka => ../../pkg/kafka/configkafka
