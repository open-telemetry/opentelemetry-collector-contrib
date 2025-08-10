module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.23.0

require (
	github.com/aws/aws-sdk-go-v2 v1.36.6
	github.com/aws/aws-sdk-go-v2/config v1.29.18
	github.com/aws/aws-sdk-go-v2/credentials v1.17.71
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.33
	github.com/aws/aws-sdk-go-v2/service/sts v1.34.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.131.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/config/confignet v1.37.1-0.20250808075651-ed09fb912b2f
	go.opentelemetry.io/collector/config/configtls v1.37.1-0.20250808075651-ed09fb912b2f
	go.uber.org/zap v1.27.0
)

require (
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.37 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.37 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.4 // indirect
	github.com/aws/smithy-go v1.22.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.37.1-0.20250808075651-ed09fb912b2f // indirect
	go.opentelemetry.io/collector/featuregate v1.37.1-0.20250808075651-ed09fb912b2f // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
