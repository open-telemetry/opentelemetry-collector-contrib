module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubernetesclusterreceiver

go 1.14

require (
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/iancoleman/strcase v0.0.0-20171129010253-3de563c3dc08
	github.com/magiconair/properties v1.8.1
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200413233902-380a0e75c518
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.14.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
)
