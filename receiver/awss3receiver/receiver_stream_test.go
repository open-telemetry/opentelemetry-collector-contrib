// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver/internal/metadata"
)

func newStreamTestReceiver(t *testing.T, processor receiverProcessor, ext component.Component) *awss3Receiver {
	t.Helper()
	settings := receivertest.NewNopSettings(metadata.Type)
	obs, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID: settings.ID, Transport: "s3", ReceiverCreateSettings: settings,
	})
	require.NoError(t, err)
	return &awss3Receiver{
		logger: zap.NewNop(), obsrecv: obs, dataProcessor: processor,
		extensions: encodingExtensions{{extension: ext, suffix: ".txt"}},
	}
}

func newTextExtension(t *testing.T) extension.Extension {
	t.Helper()
	factory := textencodingextension.NewFactory()
	ext, err := factory.Create(t.Context(), extension.Settings{
		ID: component.NewID(factory.Type()), TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}, factory.CreateDefaultConfig())
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(t.Context())) })
	return ext
}

// The producer cannot finish the object until the first batch is consumed. A
// receiver that buffers either the download or decompression deadlocks here.
func TestReceiveStreamsBeforeEOF(t *testing.T) {
	for _, compression := range []string{"none", "gzip", "zstd"} {
		t.Run(compression, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			input, output := io.Pipe()
			t.Cleanup(func() {
				_ = input.Close()
				_ = output.Close()
			})
			firstBatch := make(chan struct{})
			var batches, records int
			c, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
				batches++
				records += logs.LogRecordCount()
				if batches == 1 {
					close(firstBatch)
				}
				return nil
			})
			require.NoError(t, err)
			r := newStreamTestReceiver(t, &logsReceiver{consumer: c}, newTextExtension(t))
			key := "logs.txt"
			switch compression {
			case "gzip":
				key += ".gz"
			case "zstd":
				key += ".zst"
			}
			producerDone := make(chan error, 1)
			go func() {
				var writer io.WriteCloser = output
				flush := func() error { return nil }
				switch compression {
				case "gzip":
					gz := gzip.NewWriter(output)
					writer, flush = gz, gz.Flush
				case "zstd":
					zw, createErr := zstd.NewWriter(output)
					if createErr != nil {
						producerDone <- createErr
						return
					}
					writer, flush = zw, zw.Flush
				}
				_, writeErr := io.WriteString(writer, strings.Repeat("first\n", 1000))
				if writeErr == nil {
					writeErr = flush()
				}
				if writeErr == nil {
					select {
					case <-firstBatch:
						_, writeErr = io.WriteString(writer, "last") // No final newline.
					case <-ctx.Done():
						writeErr = ctx.Err()
					}
				}
				if writeErr == nil {
					writeErr = writer.Close()
				}
				_ = output.CloseWithError(writeErr)
				producerDone <- writeErr
			}()
			receiverDone := make(chan error, 1)
			go func() { receiverDone <- r.receive(ctx, key, input) }()
			select {
			case err = <-receiverDone:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("no batch delivered before the object finished downloading")
			}
			require.NoError(t, <-producerDone)
			require.Equal(t, 2, batches)
			require.Equal(t, 1001, records)
		})
	}
}

type streamTestExtension struct {
	nonEncodingExtension
	logs    func(io.Reader) (encoding.LogsDecoder, error)
	metrics func(io.Reader) (encoding.MetricsDecoder, error)
}

func (e streamTestExtension) NewLogsDecoder(r io.Reader, _ ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return e.logs(r)
}

func (e streamTestExtension) NewMetricsDecoder(r io.Reader, _ ...encoding.DecoderOption) (encoding.MetricsDecoder, error) {
	return e.metrics(r)
}

// These methods must not be used when a streaming decoder is available.
func (streamTestExtension) UnmarshalLogs([]byte) (plog.Logs, error) {
	panic("non-streaming unmarshaler called")
}

func (streamTestExtension) UnmarshalMetrics([]byte) (pmetric.Metrics, error) {
	panic("non-streaming unmarshaler called")
}

type streamTestDecoder struct {
	decode func() error
}

func (d streamTestDecoder) DecodeLogs() (plog.Logs, error) {
	return generateLogData(), d.decode()
}

func (d streamTestDecoder) DecodeMetrics() (pmetric.Metrics, error) {
	return generateMetricData(), d.decode()
}

func (streamTestDecoder) Offset() int64 { return 0 }

func TestReceiveStreamErrors(t *testing.T) {
	failure := errors.New("stream failure")
	for _, signal := range []string{"logs", "metrics"} {
		for _, stage := range []string{"success", "create", "decode", "read", "consumer", "cancel", "cancel_before_read"} {
			t.Run(signal+"/"+stage, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var decoded, consumed int
				newDecoder := func(reader io.Reader) (streamTestDecoder, error) {
					if stage == "create" {
						return streamTestDecoder{}, failure
					}
					return streamTestDecoder{decode: func() error {
						decoded++
						if stage == "decode" && decoded == 2 {
							return failure
						}
						var buf [1]byte
						_, err := reader.Read(buf[:])
						return err
					}}, nil
				}
				ext := streamTestExtension{
					logs:    func(r io.Reader) (encoding.LogsDecoder, error) { return newDecoder(r) },
					metrics: func(r io.Reader) (encoding.MetricsDecoder, error) { return newDecoder(r) },
				}
				consume := func() error {
					consumed++
					if stage == "consumer" {
						return failure
					}
					if stage == "cancel" {
						cancel()
					}
					return nil
				}
				var processor receiverProcessor
				if signal == "logs" {
					c, err := consumer.NewLogs(func(context.Context, plog.Logs) error { return consume() })
					require.NoError(t, err)
					processor = &logsReceiver{consumer: c}
				} else {
					c, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error { return consume() })
					require.NoError(t, err)
					processor = &metricsReceiver{consumer: c}
				}
				r := newStreamTestReceiver(t, processor, ext)
				var input io.Reader = strings.NewReader("ab")
				if stage == "read" {
					input = io.MultiReader(strings.NewReader("a"), errorReader{err: failure})
				}
				if stage == "cancel_before_read" {
					cancel()
				}
				err := r.receive(ctx, "object.txt", input)
				switch stage {
				case "success":
					require.NoError(t, err)
					require.Equal(t, 2, consumed)
				case "create":
					require.ErrorIs(t, err, failure)
					require.Zero(t, consumed)
				case "cancel_before_read":
					require.ErrorIs(t, err, context.Canceled)
					require.Zero(t, decoded)
				case "cancel":
					require.ErrorIs(t, err, context.Canceled)
					require.Equal(t, 1, decoded)
				default:
					require.ErrorIs(t, err, failure)
					require.Equal(t, 1, consumed)
				}
			})
		}
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestReceiveCompressedStreams(t *testing.T) {
	compressions := []struct {
		name     string
		suffix   string
		compress func([]byte) []byte
	}{
		{
			name:     "gzip",
			suffix:   ".gz",
			compress: gzipCompress,
		},
		{
			name:     "zstd",
			suffix:   ".zst",
			compress: zstdCompress,
		},
	}
	for _, compression := range compressions {
		t.Run(compression.name, func(t *testing.T) {
			body := compression.compress([]byte("first\n"))
			corruptBody := bytes.Clone(body)
			corruptBody[len(corruptBody)-1] ^= 0xff
			tests := []struct {
				name        string
				body        []byte
				wantErr     bool
				wantRecords int
			}{
				{
					name:        "multiple_members",
					body:        append(bytes.Clone(body), compression.compress([]byte("last\n"))...),
					wantRecords: 2,
				},
				{
					name:    "corrupt_trailer",
					body:    corruptBody,
					wantErr: true,
				},
				{
					name:    "truncated",
					body:    body[:len(body)-3],
					wantErr: true,
				},
				{
					name:        "empty",
					body:        compression.compress(nil),
					wantRecords: 0,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var records int
					c, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
						records += logs.LogRecordCount()
						return nil
					})
					require.NoError(t, err)
					r := newStreamTestReceiver(t, &logsReceiver{consumer: c}, newTextExtension(t))
					err = r.receive(t.Context(), "object.txt"+compression.suffix, bytes.NewReader(tt.body))
					if tt.wantErr {
						require.Error(t, err)
						return
					}
					require.NoError(t, err)
					require.Equal(t, tt.wantRecords, records)
				})
			}
		})
	}
}

func TestReceiveDrainsDecoderRemainder(t *testing.T) {
	failure := errors.New("download failed after decoder finished")
	ext := streamTestExtension{logs: func(io.Reader) (encoding.LogsDecoder, error) {
		return streamTestDecoder{decode: func() error { return io.EOF }}, nil
	}}
	r := newStreamTestReceiver(t, &logsReceiver{}, ext)
	err := r.receive(t.Context(), "object.txt", errorReader{err: failure})
	require.ErrorIs(t, err, failure)
}
