module github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.3.0-20200605184202-f640b7103f96
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.3.1-0.20200612184320-01ce74db9c44
	go.uber.org/zap v1.13.0
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
replace go.opentelemetry.io/collector => github.com/SumoLogic/opentelemetry-collector  v0.2.7-0.20200615151012-78b0f93ca349