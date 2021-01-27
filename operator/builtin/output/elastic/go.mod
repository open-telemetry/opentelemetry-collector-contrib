module github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/elastic

go 1.14

require (
	github.com/elastic/go-elasticsearch/v7 v7.9.0
	github.com/hashicorp/go-uuid v1.0.2
	github.com/opentelemetry/opentelemetry-log-collection v0.13.12
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.15.0
)

replace github.com/opentelemetry/opentelemetry-log-collection => ../../../../
