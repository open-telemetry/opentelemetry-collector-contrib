// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

func TestStart(t *testing.T) {
	mockConsumer := &consumertest.LogsSink{}

	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	logsReceiver, err := factory.CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(),
		factory.CreateDefaultConfig(),
		mockConsumer,
	)
	require.NoError(t, err, "receiver should successfully build")

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	stanzaReceiver := logsReceiver.(*receiver)
	logChan := stanzaReceiver.emitter.OutChannelForWrite()
	logChan <- []*entry.Entry{entry.New()}

	// Eventually because of asynchronuous nature of the receiver.
	require.Eventually(t,
		func() bool {
			return mockConsumer.LogRecordCount() == 1
		},
		10*time.Second, 5*time.Millisecond, "one log entry expected",
	)
	require.NoError(t, logsReceiver.Shutdown(context.Background()))
}

func TestHandleStartError(t *testing.T) {
	mockConsumer := &consumertest.LogsSink{}

	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	cfg := factory.CreateDefaultConfig().(*TestConfig)
	cfg.Input = NewUnstartableConfig()

	receiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(), cfg, mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = receiver.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err, "receiver fails to start under rare circumstances")
}

func TestHandleConsume(t *testing.T) {
	mockConsumer := &consumertest.LogsSink{}
	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	logsReceiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(), factory.CreateDefaultConfig(), mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	stanzaReceiver := logsReceiver.(*receiver)
	logChan := stanzaReceiver.emitter.OutChannelForWrite()
	logChan <- []*entry.Entry{entry.New()}

	// Eventually because of asynchronuous nature of the receiver.
	require.Eventually(t,
		func() bool {
			return mockConsumer.LogRecordCount() == 1
		},
		10*time.Second, 5*time.Millisecond, "one log entry expected",
	)
	require.NoError(t, logsReceiver.Shutdown(context.Background()))
}

func TestHandleConsumeRetry(t *testing.T) {
	mockConsumer := consumerretry.NewMockLogsRejecter(2)
	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	cfg := factory.CreateDefaultConfig()
	cfg.(*TestConfig).BaseConfig.RetryOnFailure.Enabled = true
	cfg.(*TestConfig).BaseConfig.RetryOnFailure.InitialInterval = 10 * time.Millisecond
	logsReceiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(), cfg, mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	require.NoError(t, logsReceiver.Start(context.Background(), componenttest.NewNopHost()))

	stanzaReceiver := logsReceiver.(*receiver)
	logChan := stanzaReceiver.emitter.OutChannelForWrite()
	logChan <- []*entry.Entry{entry.New()}

	require.Eventually(t,
		func() bool {
			return mockConsumer.LogRecordCount() == 1
		},
		1*time.Second, 5*time.Millisecond, "one log entry expected",
	)
	require.NoError(t, logsReceiver.Shutdown(context.Background()))
}

func TestShutdownFlush(t *testing.T) {
	mockConsumer := &consumertest.LogsSink{}
	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	logsReceiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(), factory.CreateDefaultConfig(), mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	var consumedLogCount atomic.Int32
	closeCh := make(chan struct{})
	stanzaReceiver := logsReceiver.(*receiver)
	go func() {
		for {
			select {
			case <-closeCh:
				assert.NoError(t, logsReceiver.Shutdown(context.Background()))
				fmt.Println(">> Shutdown called")
				return
			default:
				err := stanzaReceiver.emitter.Process(context.Background(), entry.New())
				assert.NoError(t, err)
			}
			consumedLogCount.Add(1)
		}
	}()
	require.Eventually(t, func() bool {
		return consumedLogCount.Load() > 100
	}, 5*time.Second, 5*time.Millisecond)

	close(closeCh)

	// Eventually because of asynchronuous nature of the receiver.
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			assert.Equal(t, consumedLogCount.Load(), int32(mockConsumer.LogRecordCount()))
		},
		2*time.Second, 5*time.Millisecond,
	)
}

func BenchmarkReceiver(b *testing.B) {
	b.Run(
		"1 Log entry per iteration",
		func(b *testing.B) {
			benchmarkReceiver(b, 1)
		},
	)
	b.Run(
		"10 Log entries per iteration",
		func(b *testing.B) {
			benchmarkReceiver(b, 10)
		},
	)
	b.Run(
		"100 Log entries per iteration",
		func(b *testing.B) {
			benchmarkReceiver(b, 100)
		},
	)
	b.Run(
		"1_000 Log entries per iteration",
		func(b *testing.B) {
			benchmarkReceiver(b, 1_000)
		},
	)
	b.Run(
		"10_000 Log entries per iteration",
		func(b *testing.B) {
			benchmarkReceiver(b, 10_000)
		},
	)
}

func benchmarkReceiver(b *testing.B, logsPerIteration int) {
	iterationComplete := make(chan struct{})
	nextIteration := make(chan struct{})

	inputBuilder := &testInputBuilder{
		numberOfLogEntries: logsPerIteration,
		nextIteration:      nextIteration,
	}
	inputCfg := operator.Config{
		Builder: inputBuilder,
	}

	set := componenttest.NewNopTelemetrySettings()
	emitter := helper.NewLogEmitter(set)
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     []operator.Config{inputCfg},
		DefaultOutput: emitter,
	}.Build(set)
	require.NoError(b, err)

	storageClient := storagetest.NewInMemoryClient(
		component.KindReceiver,
		component.MustNewID("foolog"),
		"test",
	)

	converter := NewConverter(set)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings()})
	require.NoError(b, err)

	mockConsumer := &testConsumer{
		receivedAllLogs: iterationComplete,
		expectedLogs:    uint32(logsPerIteration),
		receivedLogs:    atomic.Uint32{},
	}
	rcv := &receiver{
		set:           set,
		pipe:          pipe,
		emitter:       emitter,
		consumer:      mockConsumer,
		converter:     converter,
		obsrecv:       obsrecv,
		storageClient: storageClient,
	}

	b.ResetTimer()

	require.NoError(b, rcv.Start(context.Background(), nil))

	for i := 0; i < b.N; i++ {
		nextIteration <- struct{}{}
		<-iterationComplete
		mockConsumer.receivedLogs.Store(0)
	}

	require.NoError(b, rcv.Shutdown(context.Background()))
}

func BenchmarkReadLine(b *testing.B) {
	filePath := filepath.Join(b.TempDir(), "bench.log")

	pipelineYaml := fmt.Sprintf(`
- type: file_input
  include:
    - %s
  start_at: beginning`,
		filePath)

	var operatorCfgs []operator.Config
	require.NoError(b, yaml.Unmarshal([]byte(pipelineYaml), &operatorCfgs))

	set := componenttest.NewNopTelemetrySettings()
	emitter := helper.NewLogEmitter(set)
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     operatorCfgs,
		DefaultOutput: emitter,
	}.Build(set)
	require.NoError(b, err)

	// Populate the file that will be consumed
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		_, err := file.WriteString("testlog\n")
		require.NoError(b, err)
	}

	storageClient := storagetest.NewInMemoryClient(
		component.KindReceiver,
		component.MustNewID("foolog"),
		"test",
	)

	// Run the actual benchmark
	b.ResetTimer()
	require.NoError(b, pipe.Start(storageClient))
	logChan := emitter.OutChannel()
	for i := 0; i < b.N; i++ {
		entries := <-logChan
		for _, e := range entries {
			convert(e)
		}
	}
}

func BenchmarkParseAndMap(b *testing.B) {
	filePath := filepath.Join(b.TempDir(), "bench.log")

	fileInputYaml := fmt.Sprintf(`
- type: file_input
  include:
    - %s
  start_at: beginning`, filePath)

	regexParserYaml := `
- type: regex_parser
  regex: '(?P<remote_host>[^\s]+) - (?P<remote_user>[^\s]+) \[(?P<timestamp>[^\]]+)\] "(?P<http_method>[A-Z]+) (?P<path>[^\s]+)[^"]+" (?P<http_status>\d+) (?P<bytes_sent>[^\s]+)'
  timestamp:
    parse_from: timestamp
    layout: '%d/%b/%Y:%H:%M:%S %z'
  severity:
    parse_from: http_status
    preserve: true
    mapping:
      critical: 5xx
      error: 4xx
      info: 3xx
      debug: 2xx`

	pipelineYaml := fmt.Sprintf("%s%s", fileInputYaml, regexParserYaml)

	var operatorCfgs []operator.Config
	require.NoError(b, yaml.Unmarshal([]byte(pipelineYaml), &operatorCfgs))

	set := componenttest.NewNopTelemetrySettings()
	emitter := helper.NewLogEmitter(set)
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     operatorCfgs,
		DefaultOutput: emitter,
	}.Build(set)
	require.NoError(b, err)

	// Populate the file that will be consumed
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		_, err := file.WriteString(fmt.Sprintf("10.33.121.119 - - [11/Aug/2020:00:00:00 -0400] \"GET /index.html HTTP/1.1\" 404 %d\n", i%1000))
		require.NoError(b, err)
	}

	storageClient := storagetest.NewInMemoryClient(
		component.KindReceiver,
		component.MustNewID("foolog"),
		"test",
	)

	// Run the actual benchmark
	b.ResetTimer()
	require.NoError(b, pipe.Start(storageClient))
	logChan := emitter.OutChannel()
	for i := 0; i < b.N; i++ {
		entries := <-logChan
		for _, e := range entries {
			convert(e)
		}
	}
}

const testInputOperatorTypeStr = "test_input"

type testInputBuilder struct {
	numberOfLogEntries int
	nextIteration      chan struct{}
}

func (t *testInputBuilder) ID() string {
	return testInputOperatorTypeStr
}

func (t *testInputBuilder) Type() string {
	return testInputOperatorTypeStr
}

func (t *testInputBuilder) Build(settings component.TelemetrySettings) (operator.Operator, error) {
	inputConfig := helper.NewInputConfig(t.ID(), testInputOperatorTypeStr)
	inputOperator, err := inputConfig.Build(settings)
	if err != nil {
		return nil, err
	}

	return &testInputOperator{
		InputOperator:      inputOperator,
		numberOfLogEntries: t.numberOfLogEntries,
		nextIteration:      t.nextIteration,
	}, nil
}

func (t *testInputBuilder) SetID(_ string) {}

var _ operator.Operator = &testInputOperator{}

type testInputOperator struct {
	helper.InputOperator
	numberOfLogEntries int
	nextIteration      chan struct{}
	cancelFunc         context.CancelFunc
}

func (t *testInputOperator) ID() string {
	return testInputOperatorTypeStr
}

func (t *testInputOperator) Type() string {
	return testInputOperatorTypeStr
}

func (t *testInputOperator) Start(_ operator.Persister) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	t.cancelFunc = cancelFunc

	e := complexEntry()
	go func() {
		for {
			select {
			case <-t.nextIteration:
				for i := 0; i < t.numberOfLogEntries; i++ {
					_ = t.Write(context.Background(), e)
				}
			case <-ctx.Done():
				return
			}
		}

	}()
	return nil
}

func (t *testInputOperator) Stop() error {
	t.cancelFunc()
	return nil
}

type testConsumer struct {
	receivedAllLogs chan struct{}
	expectedLogs    uint32
	receivedLogs    atomic.Uint32
}

func (t *testConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (t *testConsumer) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	if t.receivedLogs.Add(uint32(ld.LogRecordCount())) >= t.expectedLogs {
		t.receivedAllLogs <- struct{}{}
	}
	return nil
}
