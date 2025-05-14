// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"gopkg.in/yaml.v3"

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
		receivertest.NewNopSettings(factory.Type()),
		factory.CreateDefaultConfig(),
		mockConsumer,
	)
	require.NoError(t, err, "receiver should successfully build")

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	stanzaReceiver := logsReceiver.(*receiver)

	stanzaReceiver.consumeEntries(context.Background(), []*entry.Entry{entry.New()})

	// Eventually because of asynchronuous nature of the receiver.
	require.Equal(t, 1, mockConsumer.LogRecordCount())

	require.NoError(t, logsReceiver.Shutdown(context.Background()))
}

func TestHandleStartError(t *testing.T) {
	mockConsumer := &consumertest.LogsSink{}

	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	cfg := factory.CreateDefaultConfig().(*TestConfig)
	cfg.Input = NewUnstartableConfig()

	receiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(factory.Type()), cfg, mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = receiver.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err, "receiver fails to start under rare circumstances")
}

func TestHandleConsume(t *testing.T) {
	mockConsumer := &consumertest.LogsSink{}
	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)

	logsReceiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig(), mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	stanzaReceiver := logsReceiver.(*receiver)

	stanzaReceiver.consumeEntries(context.Background(), []*entry.Entry{entry.New()})

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
	cfg.(*TestConfig).RetryOnFailure.Enabled = true
	cfg.(*TestConfig).RetryOnFailure.InitialInterval = 10 * time.Millisecond
	logsReceiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(factory.Type()), cfg, mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	require.NoError(t, logsReceiver.Start(context.Background(), componenttest.NewNopHost()))

	stanzaReceiver := logsReceiver.(*receiver)

	stanzaReceiver.consumeEntries(context.Background(), []*entry.Entry{entry.New()})

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

	logsReceiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig(), mockConsumer)
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

func BenchmarkReceiverWithBatchingLogEmitter(b *testing.B) {
	for n := range 6 {
		logEntries := int(math.Pow(10, float64(n)))
		b.Run(fmt.Sprintf("%d logs", logEntries), func(b *testing.B) {
			benchmarkReceiver(b, logEntries, false, true)
		})
	}
}

func BenchmarkReceiverWithSynchronousLogEmitter(b *testing.B) {
	for n := range 6 {
		logEntries := int(math.Pow(10, float64(n)))
		b.Run(fmt.Sprintf("%d logs", logEntries), func(b *testing.B) {
			benchmarkReceiver(b, logEntries, false, false)
		})
	}
}

func BenchmarkReceiverWithSynchronousLogEmitterAndBatchingInput(b *testing.B) {
	for n := range 6 {
		logEntries := int(math.Pow(10, float64(n)))
		b.Run(fmt.Sprintf("%d logs", logEntries), func(b *testing.B) {
			benchmarkReceiver(b, logEntries, true, false)
		})
	}
}

func benchmarkReceiver(b *testing.B, logsPerIteration int, batchingInput, batchingLogEmitter bool) {
	iterationComplete := make(chan struct{})
	nextIteration := make(chan struct{})

	inputBuilder := &testInputBuilder{
		numberOfLogEntries: logsPerIteration,
		nextIteration:      nextIteration,
		produceBatches:     batchingInput,
	}
	inputCfg := operator.Config{
		Builder: inputBuilder,
	}

	storageClient := storagetest.NewInMemoryClient(
		component.KindReceiver,
		component.MustNewID("foolog"),
		"test",
	)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(component.MustNewType("foolog"))})
	require.NoError(b, err)

	mockConsumer := &testConsumer{
		receivedAllLogs: iterationComplete,
		expectedLogs:    uint32(logsPerIteration),
		receivedLogs:    atomic.Uint32{},
	}
	rcv := &receiver{
		consumer:      mockConsumer,
		obsrecv:       obsrecv,
		storageClient: storageClient,
	}

	set := componenttest.NewNopTelemetrySettings()
	var emitter helper.LogEmitter
	if batchingLogEmitter {
		emitter = helper.NewBatchingLogEmitter(set, rcv.consumeEntries)
	} else {
		emitter = helper.NewSynchronousLogEmitter(set, rcv.consumeEntries)
	}
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     []operator.Config{inputCfg},
		DefaultOutput: emitter,
	}.Build(set)
	require.NoError(b, err)

	rcv.pipe = pipe
	rcv.set = set
	rcv.emitter = emitter

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
	receivedAllLogs := make(chan struct{})
	filePath := filepath.Join(b.TempDir(), "bench.log")

	pipelineYaml := fmt.Sprintf(`
pipeline:
  type: file_input
  include:
    - %s
  start_at: beginning`,
		filePath)

	confmapFilePath := filepath.Join(b.TempDir(), "conf.yaml")
	require.NoError(b, os.WriteFile(confmapFilePath, []byte(pipelineYaml), 0o600))

	testConfMaps, err := confmaptest.LoadConf(confmapFilePath)
	require.NoError(b, err)

	conf, err := testConfMaps.Sub("pipeline")
	require.NoError(b, err)
	require.NotNil(b, conf)

	operatorCfg := operator.Config{}
	require.NoError(b, conf.Unmarshal(&operatorCfg))

	operatorCfgs := []operator.Config{operatorCfg}

	storageClient := storagetest.NewInMemoryClient(
		component.KindReceiver,
		component.MustNewID("foolog"),
		"test",
	)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(component.MustNewType("foolog"))})
	require.NoError(b, err)

	mockConsumer := &testConsumer{
		receivedAllLogs: receivedAllLogs,
		expectedLogs:    uint32(b.N),
		receivedLogs:    atomic.Uint32{},
	}
	rcv := &receiver{
		consumer:      mockConsumer,
		obsrecv:       obsrecv,
		storageClient: storageClient,
	}

	set := componenttest.NewNopTelemetrySettings()
	emitter := helper.NewBatchingLogEmitter(set, rcv.consumeEntries)
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     operatorCfgs,
		DefaultOutput: emitter,
	}.Build(set)
	require.NoError(b, err)

	rcv.pipe = pipe
	rcv.set = set
	rcv.emitter = emitter

	// Populate the file that will be consumed
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0o666)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		_, err := file.WriteString("testlog\n")
		require.NoError(b, err)
	}

	// Run the actual benchmark
	b.ResetTimer()
	require.NoError(b, rcv.Start(context.Background(), nil))

	<-receivedAllLogs

	require.NoError(b, rcv.Shutdown(context.Background()))
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
	emitter := helper.NewBatchingLogEmitter(set, func(_ context.Context, entries []*entry.Entry) {
		for _, e := range entries {
			convert(e)
		}
	})
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     operatorCfgs,
		DefaultOutput: emitter,
	}.Build(set)
	require.NoError(b, err)

	// Populate the file that will be consumed
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0o666)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		_, err := fmt.Fprintf(file, "10.33.121.119 - - [11/Aug/2020:00:00:00 -0400] \"GET /index.html HTTP/1.1\" 404 %d\n", i%1000)
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
}

const testInputOperatorTypeStr = "test_input"

type testInputBuilder struct {
	numberOfLogEntries int
	nextIteration      chan struct{}
	produceBatches     bool
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
		produceBatches:     t.produceBatches,
		nextIteration:      t.nextIteration,
	}, nil
}

func (t *testInputBuilder) SetID(_ string) {}

var _ operator.Operator = &testInputOperator{}

type testInputOperator struct {
	helper.InputOperator
	numberOfLogEntries int
	produceBatches     bool
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
	go func(writeBatches bool) {
		for {
			select {
			case <-t.nextIteration:
				if writeBatches {
					for i := 0; i < t.numberOfLogEntries; i += len(entries) {
						_ = t.WriteBatch(context.Background(), entries)
					}
				} else {
					for i := 0; i < t.numberOfLogEntries; i++ {
						_ = t.Write(context.Background(), e)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}(t.produceBatches)
	return nil
}

var entries = complexEntriesForNDifferentHosts(100, 4)

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
