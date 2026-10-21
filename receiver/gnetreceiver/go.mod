module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnetreceiver

go 1.26.0

require (
	github.com/panjf2000/gnet/v2 v2.7.1
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.69.1-0.20261015205720-932ee30af2c0
	go.opentelemetry.io/collector/consumer v1.69.1-0.20261015205720-932ee30af2c0
	go.opentelemetry.io/collector/consumer/consumertest v0.163.1-0.20261015205720-932ee30af2c0
	go.opentelemetry.io/collector/pdata v1.69.1-0.20261015205720-932ee30af2c0
	go.opentelemetry.io/collector/receiver v1.69.1-0.20261015205720-932ee30af2c0
	go.opentelemetry.io/collector/receiver/receivertest v0.163.1-0.20261015205720-932ee30af2c0
	go.uber.org/zap v1.28.0
)
