// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	// "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"kythe.io/kythe/go/util/riegeli"
)

// stringReader is the only function we use from *bufio.Reader. We define it
// so that it can be swapped out for testing.
type stringReader interface {
	ReadString(delim byte) (string, error)
}
type unmarshaler struct {
	metricsUnm pmetric.Unmarshaler
	tracesUnm  ptrace.Unmarshaler
	logsUnm    plog.Unmarshaler
}

// fileReader
type fileReader struct {
	stringReader stringReader
	unmarshaler  unmarshaler
	consumer     consumerType
	timer        *replayTimer
	ioread       *bufio.Reader
}

func newFileReader(consumer consumerType, file *os.File, timer *replayTimer, format string) fileReader {
	mt.Println("file_reader.go:55: NEW FILE READER")
	bts := make([]byte, 10000)
	fmt.Println(file.Read(bts))
	fmt.Println(binary.BigEndian.Uint64(bts[:8]))
	fr := fileReader{
		consumer:     consumer,
		stringReader: bufio.NewReader(file),
		timer:        timer,
		ioread:       bufio.NewReader(file),
	}

	if format == formatTypeProto {
		switch {
		case fr.consumer.tracesConsumer != nil:
			fr.unmarshaler.tracesUnm = &ptrace.ProtoUnmarshaler{}
		case fr.consumer.logsConsumer != nil:
			fr.unmarshaler.logsUnm = &plog.ProtoUnmarshaler{}
		case fr.consumer.metricsConsumer != nil:
			fr.unmarshaler.metricsUnm = &pmetric.ProtoUnmarshaler{}
		}
	} else { // default to json
		switch {
		case fr.consumer.tracesConsumer != nil:
			fr.unmarshaler.tracesUnm = &ptrace.JSONUnmarshaler{}
		case fr.consumer.logsConsumer != nil:
			fr.unmarshaler.logsUnm = &plog.JSONUnmarshaler{}
		case fr.consumer.metricsConsumer != nil:
			fr.unmarshaler.metricsUnm = &pmetric.JSONUnmarshaler{}
		}
	}
	fmt.Println("file_reader.go:85: PEEK AT FIRST 8 BYTES")
	bt, _ := fr.ioread.Peek(8)
	fmt.Println(bt)
	rr := riegeli.NewReader(file)
	buf, _ := rr.Next()
	fmt.Println("file_reader.go:90: THIS IS THE RIEGELI RECORD")
	fmt.Println(buf)

	return fr
}

func (fr fileReader) readProto(_ context.Context) error {
	rr := riegeli.NewReader(fr.ioread)
	buf, err := rr.Next()
	fmt.Println("file_reader.go:99: THIS IS A RECORD FROM READER")
	fmt.Println(buf)
	fmt.Println(err)
	if err != nil {
		return err
	}
	return nil

}

// readAll calls readline for each line in the file until all lines have been
// read or the context is cancelled.
func (fr fileReader) readAll(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var err error
			switch {
			case fr.consumer.tracesConsumer != nil:
				err = fr.readTraceLine(ctx)
			case fr.consumer.metricsConsumer != nil:
				err = fr.readMetricLine(ctx)
			case fr.consumer.logsConsumer != nil:
				err = fr.readLogLine(ctx)
			}

			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	}
}

// readLogLine reads the next line in the file, converting it into logs and
// passing it to the the consumer member.
func (fr fileReader) readLogLine(ctx context.Context) error {
	line, err := fr.stringReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read line from input file: %w", err)
	}
	logs, err := fr.unmarshaler.logsUnm.UnmarshalLogs([]byte(line))
	if err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}
	err = fr.timer.wait(ctx, getFirstTimestampFromLogs(logs))
	if err != nil {
		return fmt.Errorf("readLine interrupted while waiting for timer: %w", err)
	}
	return fr.consumer.logsConsumer.ConsumeLogs(ctx, logs)
}

// readTraceLine reads the next line in the file, converting it into traces and
// passing it to the the consumer member.
func (fr fileReader) readTraceLine(ctx context.Context) error {
	line, err := fr.stringReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read line from input file: %w", err)
	}
	traces, err := fr.unmarshaler.tracesUnm.UnmarshalTraces([]byte(line))
	if err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}
	err = fr.timer.wait(ctx, getFirstTimestampFromTraces(traces))
	if err != nil {
		return fmt.Errorf("readLine interrupted while waiting for timer: %w", err)
	}
	return fr.consumer.tracesConsumer.ConsumeTraces(ctx, traces)
}

// readMetricLine reads the next line in the file, converting it into metrics and
// passing it to the the consumer member.
func (fr fileReader) readMetricLine(ctx context.Context) error {
	line, err := fr.stringReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read line from input file: %w", err)
	}
	metrics, err := fr.unmarshaler.metricsUnm.UnmarshalMetrics([]byte(line))
	if err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}
	err = fr.timer.wait(ctx, getFirstTimestampFromMetrics(metrics))
	if err != nil {
		return fmt.Errorf("readLine interrupted while waiting for timer: %w", err)
	}
	return fr.consumer.metricsConsumer.ConsumeMetrics(ctx, metrics)
}

func getFirstTimestampFromLogs(logs plog.Logs) pcommon.Timestamp {
	resourceLogs := logs.ResourceLogs()
	if resourceLogs.Len() == 0 {
		return 0
	}
	scopeLogs := resourceLogs.At(0).ScopeLogs()
	if scopeLogs.Len() == 0 {
		return 0
	}
	logSlice := scopeLogs.At(0).LogRecords()
	if logSlice.Len() == 0 {
		return 0
	}

	return logSlice.At(0).Timestamp()
}

func getFirstTimestampFromTraces(traces ptrace.Traces) pcommon.Timestamp {
	resourceSpans := traces.ResourceSpans()
	if resourceSpans.Len() == 0 {
		return 0
	}
	scopeSpans := resourceSpans.At(0).ScopeSpans()
	if scopeSpans.Len() == 0 {
		return 0
	}
	spanSlice := scopeSpans.At(0).Spans()
	if spanSlice.Len() == 0 {
		return 0
	}

	return spanSlice.At(0).StartTimestamp()
}

func getFirstTimestampFromMetrics(metrics pmetric.Metrics) pcommon.Timestamp {
	resourceMetrics := metrics.ResourceMetrics()
	if resourceMetrics.Len() == 0 {
		return 0
	}
	scopeMetrics := resourceMetrics.At(0).ScopeMetrics()
	if scopeMetrics.Len() == 0 {
		return 0
	}
	metricSlice := scopeMetrics.At(0).Metrics()
	if metricSlice.Len() == 0 {
		return 0
	}

	metric := metricSlice.At(0)
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		if dps.Len() == 0 {
			return 0
		}
		return dps.At(0).Timestamp()
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		if dps.Len() == 0 {
			return 0
		}
		return dps.At(0).Timestamp()
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		if dps.Len() == 0 {
			return 0
		}
		return dps.At(0).Timestamp()
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		if dps.Len() == 0 {
			return 0
		}
		return dps.At(0).Timestamp()
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		if dps.Len() == 0 {
			return 0
		}
		return dps.At(0).Timestamp()
	}
	return 0
}
