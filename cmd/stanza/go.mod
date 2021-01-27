module github.com/opentelemetry/opentelemetry-log-collection/cmd/stanza

go 1.14

require (
	github.com/kardianos/service v1.2.0
	github.com/opentelemetry/opentelemetry-log-collection v0.13.12
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/k8sevent v0.1.0
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/windows v0.1.1
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/elastic v0.1.2
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/googlecloud v0.1.2
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/newrelic v0.1.2
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/otlp v0.1.0
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/syslog v0.1.3
	github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/k8smetadata v0.1.2
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.6.1
	go.etcd.io/bbolt v1.3.5
	go.uber.org/zap v1.16.0
)

replace github.com/opentelemetry/opentelemetry-log-collection => ../../

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/k8sevent => ../../operator/builtin/input/k8sevent

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/windows => ../../operator/builtin/input/windows

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/syslog => ../../operator/builtin/parser/syslog

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/transformer/k8smetadata => ../../operator/builtin/transformer/k8smetadata

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/elastic => ../../operator/builtin/output/elastic

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/googlecloud => ../../operator/builtin/output/googlecloud

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/newrelic => ../../operator/builtin/output/newrelic

replace github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/otlp => ../../operator/builtin/output/otlp
