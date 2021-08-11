module example.com/awss3configsource

go 1.16

require (
	github.com/aws/aws-sdk-go v1.40.5
	github.com/signalfx/splunk-otel-collector v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/collector v0.31.1-0.20210810171211-8038673eba9e
	go.uber.org/zap v1.19.0
)

replace github.com/signalfx/splunk-otel-collector => ../configprovider
