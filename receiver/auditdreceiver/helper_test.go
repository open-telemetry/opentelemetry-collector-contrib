//go:build linux
// +build linux

package auditdreceiver

import (
	"fmt"
	"iter"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
)

var (
	TEST_PATH                   = "testdata"
	TEST_CONFIG_PATH            = "config.yaml"
)

func beforeEach[A testing.TB](t A, should_create_inner_dir bool) (receiver.Logs, *consumertest.LogsSink, *AuditdReceiverConfig, string) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	test_dir := gofakeit.LetterN(5)

	root_dir := filepath.Join(wd, "testdata", test_dir)
	err = os.Mkdir(root_dir, 0o777)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(1000 * time.Millisecond)

	config := createDefaultConfig()
	config.(*AuditdReceiverConfig).Rules = []string{}

	testLogsConsumer := new(consumertest.LogsSink)
	settings := receivertest.NewNopSettings(component.MustNewType("auditd"))
	settings.Logger = zap.NewNop()
	logs, err := createLogsReceiver(t.Context(), settings, config, testLogsConsumer)
	require.NoError(t, err)
	require.NoError(t, logs.Start(t.Context(), componenttest.NewNopHost()))

	return logs, testLogsConsumer, config.(*AuditdReceiverConfig), root_dir
}

func testTeardown[A testing.TB](tb A, test_destination string) {
	// tb.Logf("removing test_destination: %v", test_destination)
	err := os.RemoveAll(test_destination)
	if err != nil {
		tb.Fatal(err)
	}
}

// logsToMap will take a list of logs and each LogRecord to a map which will count distinct events (up to: Path and Operation).
// This is useful if testing the out-of-order arrival of log records between expected and actual consumers. Solves issue with ignoring order.
func logsIterator(logs []plog.Logs) iter.Seq[plog.LogRecord] {
	return func(yield func(plog.LogRecord) bool) {
		for _, log := range logs {
			for i := 0; i < log.ResourceLogs().Len(); i++ {
				for j := 0; j < log.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
					if !yield(log.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().At(0)) {
						return
					}
				}
			}
		}
	}
}

// stats takes a bunch of floats and calculates some stats
func stats[T int64 | float64](data []T) (average, q95 T) {
	if len(data) == 0 {
		return 0, 0
	}
	slices.Sort(data)
	var sum T
	for _, x := range data {
		sum += x
	}
	x := 1 + float64(len(data)-1)*0.95
	fx := math.Floor(x)
	cx := math.Ceil(x)
	if x == fx && fx == cx {
		return sum / T(float64(len(data))), data[int(x)]
	}
	return sum / T(float64(len(data))), data[int(fx)] + T(x-fx)*(data[int(cx)]-data[int(fx)])
}

// logsObsTimestampDiffTimstamp will for each event compare the delta between Timestamp and observedTimestamp.
// it will also return some basic stats.
// Useful for calculating processing times of the receiver.
func logsObsTimestampDiffTimstamp(logs []plog.Logs) []float64 {
	ret := make([]float64, 0)
	for lr := range logsIterator(logs) {
		ret = append(ret, lr.ObservedTimestamp().AsTime().Sub(lr.Timestamp().AsTime()).Seconds())
	}
	return ret
}

// logsToMap will take a list of logs and each LogRecord to a map which will count distinct events (up to: Path and Operation).
// This is useful if testing the out-of-order arrival of log records between expected and actual consumers. Solves issue with ignoring order.
func logsToMap[A testing.TB](tb A, logs []plog.Logs, msgs ...interface{}) map[string]uint {
	ret := make(map[string]uint)
	for lr := range logsIterator(logs) {
		// We ignore the timestamp entry for each log record as this is not trivial to check for equality on.
		path, _ := lr.Attributes().Get("path")
		operation, _ := lr.Attributes().Get("operation")
		hash := fmt.Sprintf("%s-%s", filepath.Base(path.AsString()), operation.AsString())
		// tb.Logf("%s, hash=%v", msgs, hash)
		ret[hash] += 1
	}
	return ret
}

func consumeLogs[A testing.TB](tb A, consumer *consumertest.LogsSink, logs []plog.Logs) {
	for _, log := range logs {
		consumer.ConsumeLogs(tb.Context(), log)
	}
}
