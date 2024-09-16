package datareceivers

import (
	"context"
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
)

type StanzaDataReceiver struct {
	testbed.DataReceiverBase
	logReceiver  receiver.Logs
	path         string
	retry        string
	sendingQueue string
}

var _ testbed.DataReceiver = (*StanzaDataReceiver)(nil)

// NewFileLogWriter creates a new data sender that will write log entries to a
// file, to be tailed by FluentBit and sent to the collector.
func NewFileLogReceiver() *StanzaDataReceiver {
	file, err := os.CreateTemp("", "perf-logs.log")
	if err != nil {
		panic("failed to create temp file")
	}

	f := &StanzaDataReceiver{
		path: file.Name(),
	}

	return f
}
func NewLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{
		"/Users/vihasmakwana/Desktop/Vihas/OTeL/opentelemetry-collector-contrib/testbed/datareceivers/hello.log",
	}
	return cfg.Build()
}
func (s *StanzaDataReceiver) Start(tc consumer.Traces, mc consumer.Metrics, lc consumer.Logs) error {
	factory := filelogreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*filelogreceiver.FileLogConfig)
	cfg.InputConfig.Include = []string{s.path}
	cfg.InputConfig.StartAt = "beginning"
	// cfg.RetryOnFailure = consumerretry.NewDefaultConfig()
	// cfg.RetryOnFailure.Enabled = true
	var err error
	set := receivertest.NewNopSettings()
	logger, _ := NewLogger()
	set.TelemetrySettings.Logger = logger
	if s.logReceiver, err = factory.CreateLogsReceiver(context.Background(), set, cfg, lc); err != nil {
		return err
	}
	return s.logReceiver.Start(context.Background(), componenttest.NewNopHost())
}

func (*StanzaDataReceiver) Stop() error {
	return nil
}

func (s *StanzaDataReceiver) GenConfigYAMLStr() string {
	config := fmt.Sprintf(`
  file:
    path: %s
    %s
    %s
`, s.path, s.retry, s.sendingQueue)
	return config
}

func (*StanzaDataReceiver) ProtocolName() string {
	return "file"
}

func (bor *StanzaDataReceiver) WithRetry(retry string) *StanzaDataReceiver {
	bor.retry = retry
	return bor
}

func (bor *StanzaDataReceiver) WithQueue(sendingQueue string) *StanzaDataReceiver {
	bor.sendingQueue = sendingQueue
	return bor
}
