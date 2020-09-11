module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator

go 1.14

require (
	github.com/antonmedv/expr v1.8.9
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0-00010101000000-000000000000
	github.com/spf13/cast v1.3.1
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.9.1-0.20200903224024-3eb3b664a832
	go.uber.org/zap v1.16.0
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	k8s.io/client-go v0.18.6 // indirect
	k8s.io/utils v0.0.0-20200724153422-f32512634ab7 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../../extension/observer
