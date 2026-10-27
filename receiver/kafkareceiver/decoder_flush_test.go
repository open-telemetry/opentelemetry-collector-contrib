// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

func TestReceiver_StreamingDecoderFlushOptions(t *testing.T) {
	for _, signal := range []string{"logs", "metrics", "traces", "profiles"} {
		t.Run(signal, func(t *testing.T) {
			_, cfg := mustNewFakeCluster(t)
			ext := &flushAwareExtension{}
			received := make(chan decodedMessage, 1)
			r := newDecoderTestReceiver(t, signal, cfg, received)
			require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): ext}))
			t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

			for i := range 2 {
				id := strconv.Itoa(i)
				require.NoError(t, r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
					Topic: "messages", Value: []byte("value"),
					Headers: []kgo.RecordHeader{{Key: "tenant", Value: []byte(id)}},
				}, attribute.NewSet()))
				assert.Equal(t, 1, awaitDecoderValue(t, received).count)
			}
			require.Len(t, ext.options, 2)
			for i, options := range ext.options {
				assert.Zero(t, options.FlushItems, "Kafka message boundaries must determine decoding boundaries")
				assert.Zero(t, options.FlushBytes, "Kafka messages must not be split at the default byte threshold")
				assert.Equal(t, []string{strconv.Itoa(i)}, client.FromContext(options.Context).Metadata.Get("tenant"),
					"disabling flush thresholds must preserve the current message context")
			}
		})
	}
}

func TestReceiver_StreamingDecoderDoesNotFlushMessage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		items   int
	}{
		{
			name: "item limit", payload: strings.Repeat("record\n", 1000) + "record", items: 1001,
		},
		{
			name: "byte limit", payload: strings.Repeat("x", 1<<20) + "\nrecord", items: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, cfg := mustNewFakeCluster(t)
			ext := &flushAwareExtension{}
			received := make(chan decodedMessage, 1)
			r := newDecoderTestReceiver(t, "logs", cfg, received)
			require.NoError(t, r.Start(t.Context(), extensionsHost{component.MustNewID("context"): ext}))
			t.Cleanup(func() { assert.NoError(t, r.Shutdown(context.Background())) }) //nolint:usetesting

			// Each payload exceeds one default flush threshold before its last
			// record. Without disabling that threshold, the decoder would emit
			// an intermediate batch that Kafka must accumulate back together.
			require.NoError(t, r.(*franzConsumer).consumeMessage(t.Context(), &kgo.Record{
				Topic: "messages", Value: []byte(tc.payload),
			}, attribute.NewSet()))
			assert.Equal(t, tc.items, awaitDecoderValue(t, received).count)
			assert.Equal(t, 1, ext.emittedBatches, "the decoder must emit the complete Kafka message without an intermediate flush")
		})
	}
}

// flushAwareExtension records options at each factory boundary. Logs decoding
// additionally honors both flush thresholds, like the streaming helpers do.
type flushAwareExtension struct {
	messageContextExtension
	options        []encoding.DecoderOptions
	emittedBatches int
}

func (e *flushAwareExtension) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	opts := encoding.NewDecoderOptions(options...)
	e.options = append(e.options, opts)
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return &flushAwareLogsDecoder{
		streamingLogsDecoder: streamingLogsDecoder{batches: strings.Split(string(payload), "\n")},
		options:              opts,
		emittedBatches:       &e.emittedBatches,
	}, nil
}

func (e *flushAwareExtension) NewMetricsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.MetricsDecoder, error) {
	e.options = append(e.options, encoding.NewDecoderOptions(options...))
	return e.messageContextExtension.NewMetricsDecoder(reader, options...)
}

func (e *flushAwareExtension) NewTracesDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.TracesDecoder, error) {
	e.options = append(e.options, encoding.NewDecoderOptions(options...))
	return e.messageContextExtension.NewTracesDecoder(reader, options...)
}

func (e *flushAwareExtension) NewProfilesDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.ProfilesDecoder, error) {
	e.options = append(e.options, encoding.NewDecoderOptions(options...))
	return e.messageContextExtension.NewProfilesDecoder(reader, options...)
}

type flushAwareLogsDecoder struct {
	streamingLogsDecoder
	options        encoding.DecoderOptions
	emittedBatches *int
}

func (d *flushAwareLogsDecoder) DecodeLogs() (plog.Logs, error) {
	if d.next == len(d.batches) {
		return plog.Logs{}, io.EOF
	}
	data := plog.NewLogs()
	logs := data.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	var bytes int64
	for d.next < len(d.batches) {
		line := d.batches[d.next]
		logs.AppendEmpty().Body().SetStr(line)
		d.next++
		bytes += int64(len(line) + 1)
		if d.options.FlushItems > 0 && int64(logs.Len()) >= d.options.FlushItems ||
			d.options.FlushBytes > 0 && bytes >= d.options.FlushBytes {
			break
		}
	}
	*d.emittedBatches++
	return data, nil
}
