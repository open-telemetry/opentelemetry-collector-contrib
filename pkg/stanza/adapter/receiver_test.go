// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

func TestStart(t *testing.T) {
	mockConsumer := &consumertest.LogsSink{}

	factory := NewFactory(TestReceiverType{})

	logsReceiver, err := factory.CreateLogsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		factory.CreateDefaultConfig(),
		mockConsumer,
	)
	require.NoError(t, err, "receiver should successfully build")

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	stanzaReceiver := logsReceiver.(*receiver)
	stanzaReceiver.emitter.logChan <- []*entry.Entry{entry.New()}

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

	factory := NewFactory(TestReceiverType{})

	cfg := factory.CreateDefaultConfig().(*TestConfig)
	cfg.Input = newUnstartableParams()

	receiver, err := factory.CreateLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = receiver.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err, "receiver fails to start under rare circumstances")
}

func TestHandleConsumeError(t *testing.T) {
	mockConsumer := &mockLogsRejecter{}
	factory := NewFactory(TestReceiverType{})

	logsReceiver, err := factory.CreateLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), factory.CreateDefaultConfig(), mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	stanzaReceiver := logsReceiver.(*receiver)
	stanzaReceiver.emitter.logChan <- []*entry.Entry{entry.New()}

	// Eventually because of asynchronuous nature of the receiver.
	require.Eventually(t,
		func() bool {
			return mockConsumer.LogRecordCount() == 1
		},
		10*time.Second, 5*time.Millisecond, "one log entry expected",
	)
	require.NoError(t, logsReceiver.Shutdown(context.Background()))
}

func BenchmarkReadLine(b *testing.B) {
	filePath := filepath.Join(b.TempDir(), "bench.log")

	pipelineYaml := fmt.Sprintf(`
- type: file_input
  include:
    - %s
  start_at: beginning`,
		filePath)

	operatorCfgs := []operator.Config{}
	require.NoError(b, yaml.Unmarshal([]byte(pipelineYaml), &operatorCfgs))

	emitter := NewLogEmitter(
		LogEmitterWithLogger(zap.NewNop().Sugar()),
	)
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     operatorCfgs,
		DefaultOutput: emitter,
	}.Build(zap.NewNop().Sugar())
	require.NoError(b, err)

	// Populate the file that will be consumed
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		_, err := file.WriteString("testlog\n")
		require.NoError(b, err)
	}

	// // Run the actual benchmark
	b.ResetTimer()
	require.NoError(b, pipe.Start(newMockPersister()))
	for i := 0; i < b.N; i++ {
		entries := <-emitter.logChan
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

	operatorCfgs := []operator.Config{}
	require.NoError(b, yaml.Unmarshal([]byte(pipelineYaml), &operatorCfgs))

	emitter := NewLogEmitter(
		LogEmitterWithLogger(zap.NewNop().Sugar()),
	)
	defer func() {
		require.NoError(b, emitter.Stop())
	}()

	pipe, err := pipeline.Config{
		Operators:     operatorCfgs,
		DefaultOutput: emitter,
	}.Build(zap.NewNop().Sugar())
	require.NoError(b, err)

	// Populate the file that will be consumed
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		_, err := file.WriteString(fmt.Sprintf("10.33.121.119 - - [11/Aug/2020:00:00:00 -0400] \"GET /index.html HTTP/1.1\" 404 %d\n", i%1000))
		require.NoError(b, err)
	}

	// // Run the actual benchmark
	b.ResetTimer()
	require.NoError(b, pipe.Start(newMockPersister()))
	for i := 0; i < b.N; i++ {
		entries := <-emitter.logChan
		for _, e := range entries {
			convert(e)
		}
	}
}
