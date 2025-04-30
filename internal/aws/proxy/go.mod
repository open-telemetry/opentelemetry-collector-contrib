module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.23.0

require (
	github.com/aws/aws-sdk-go v1.55.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.124.1
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/config/confignet v1.30.0
	go.opentelemetry.io/collector/config/configtls v1.30.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws v0.0.0-00010101000000-000000000000 // indirect
	github.com/aws/aws-sdk-go-v2 v1.36.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.14 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.67 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.19 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.30.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.30.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ../../../internal/aws/awsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

replace github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws => ../../../override/aws

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
