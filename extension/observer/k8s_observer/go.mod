module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8s

go 1.14

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200413233902-380a0e75c518
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0
	github.com/stretchr/testify v1.4.0
	github.com/uber-go/atomic v1.4.0
	go.uber.org/zap v1.10.0
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../
