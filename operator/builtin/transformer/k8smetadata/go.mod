module github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/k8smetadata

go 1.14

require (
	github.com/opentelemetry/opentelemetry-log-collection v0.13.12
	github.com/stretchr/testify v1.6.1
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111 // indirect
)

replace github.com/opentelemetry/opentelemetry-log-collection => ../../../../
