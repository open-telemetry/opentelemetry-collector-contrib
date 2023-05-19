// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// stringReader is the only function we use from *bufio.Reader. We define it
// so that it can be swapped out for testing.
type stringReader interface {
	ReadString(delim byte) (string, error)
	Read(buf []byte) (int, error)
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
}

func newFileReader(consumer consumerType, file *os.File, timer *replayTimer, format string, compression string) fileReader {
	fr := fileReader{
		consumer: consumer,
		timer:    timer,
	}

	if compression == compressionTypeZSTD {
		cr, _ := zstd.NewReader(file)
		fr.stringReader = bufio.NewReader(cr)
	} else { // no compression
		fr.stringReader = bufio.NewReader(file)
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

	return fr
}

// readAllLines calls readline for each line in the file until all lines have been
// read or the context is cancelled.
func (fr fileReader) readAllLines(ctx context.Context) error {
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
		return fmt.Errorf("failed to unmarshal logs: %w", err)
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
		return fmt.Errorf("failed to unmarshal traces: %w", err)
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

// readAllChunks reads the next chunk of data where each chunk is prefixed with
// the size of the data chunk.
func (fr fileReader) readAllChunks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var err error
			switch {
			case fr.consumer.tracesConsumer != nil:
				err = fr.readTraceChunk(ctx)
			case fr.consumer.metricsConsumer != nil:
				err = fr.readMetricChunk(ctx)
			case fr.consumer.logsConsumer != nil:
				err = fr.readLogChunk(ctx)
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

func (fr fileReader) readMetricChunk(ctx context.Context) error {
	var sz int32
	err := binary.Read(fr.stringReader, binary.BigEndian, &sz)
	if err != nil {
		return err
	}
	dataBuffer := make([]byte, sz)
	err = binary.Read(fr.stringReader, binary.BigEndian, &dataBuffer)
	if err != nil {
		return fmt.Errorf("failed to read line from input file: %w", err)
	}
	metrics, err := fr.unmarshaler.metricsUnm.UnmarshalMetrics(dataBuffer)
	if err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}
	err = fr.timer.wait(ctx, getFirstTimestampFromMetrics(metrics))
	if err != nil {
		return fmt.Errorf("readLine interrupted while waiting for timer: %w", err)
	}
	return fr.consumer.metricsConsumer.ConsumeMetrics(ctx, metrics)
}

func (fr fileReader) readTraceChunk(ctx context.Context) error {
	var sz int32
	err := binary.Read(fr.stringReader, binary.BigEndian, &sz)
	if err != nil {
		return err
	}

	dataBuffer := make([]byte, sz)
	err = binary.Read(fr.stringReader, binary.BigEndian, &dataBuffer)
	if err != nil {
		return fmt.Errorf("failed to read line from input file: %w", err)
	}

	traces, err := fr.unmarshaler.tracesUnm.UnmarshalTraces(dataBuffer)
	if err != nil {
		return fmt.Errorf("failed to unmarshal traces: %w", err)
	}
	err = fr.timer.wait(ctx, getFirstTimestampFromTraces(traces))
	if err != nil {
		return fmt.Errorf("readLine interrupted while waiting for timer: %w", err)
	}
	return fr.consumer.tracesConsumer.ConsumeTraces(ctx, traces)
}

func (fr fileReader) readLogChunk(ctx context.Context) error {
	var sz int32
	err := binary.Read(fr.stringReader, binary.BigEndian, &sz)
	if err != nil {
		return err
	}

	dataBuffer := make([]byte, sz)
	err = binary.Read(fr.stringReader, binary.BigEndian, &dataBuffer)
	if err != nil {
		return fmt.Errorf("failed to read line from input file: %w", err)
	}

	logs, err := fr.unmarshaler.logsUnm.UnmarshalLogs(dataBuffer)
	if err != nil {
		return fmt.Errorf("failed to unmarshal logs: %w", err)
	}
	err = fr.timer.wait(ctx, getFirstTimestampFromLogs(logs))
	if err != nil {
		return fmt.Errorf("readLine interrupted while waiting for timer: %w", err)
	}
	return fr.consumer.logsConsumer.ConsumeLogs(ctx, logs)
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
