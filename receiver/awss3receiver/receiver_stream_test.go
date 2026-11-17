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
	"testing/iotest"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
		ReceiverID:             settings.ID,
		Transport:              "s3",
		ReceiverCreateSettings: settings,
	})
	require.NoError(t, err)
	return &awss3Receiver{
		logger:        zap.NewNop(),
		obsrecv:       obs,
		dataProcessor: processor,
		extensions: encodingExtensions{{
			extension: ext,
			suffix:    ".txt",
		}},
	}
}

func newTextExtension(t *testing.T) extension.Extension {
	t.Helper()
	factory := textencodingextension.NewFactory()
	ext, err := factory.Create(t.Context(), extension.Settings{
		ID:                component.NewID(factory.Type()),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}, factory.CreateDefaultConfig())
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(t.Context())) })
	return ext
}

// The producer cannot finish the object until the first batch is consumed. A
// receiver that buffers either the download or decompression deadlocks here.
func TestReceiveStreamsBeforeEOF(t *testing.T) {
	tests := []struct {
		name      string
		suffix    string
		newWriter func(io.Writer) (io.WriteCloser, func() error, error)
	}{
		{
			name: "none",
		},
		{
			name:   "gzip",
			suffix: ".gz",
			newWriter: func(output io.Writer) (io.WriteCloser, func() error, error) {
				writer := gzip.NewWriter(output)
				return writer, writer.Flush, nil
			},
		},
		{
			name:   "zstd",
			suffix: ".zst",
			newWriter: func(output io.Writer) (io.WriteCloser, func() error, error) {
				writer, err := zstd.NewWriter(output)
				if err != nil {
					return nil, nil, err
				}
				return writer, writer.Flush, nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			key := "logs.txt" + tt.suffix
			producerDone := make(chan error, 1)
			go func() {
				var writer io.WriteCloser = output
				flush := func() error { return nil }
				if tt.newWriter != nil {
					var createErr error
					writer, flush, createErr = tt.newWriter(output)
					if createErr != nil {
						_ = output.CloseWithError(createErr)
						producerDone <- createErr
						return
					}
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
	tests := []struct {
		name               string
		input              string
		createErr          error
		decodeErr          error
		readErr            error
		consumerErr        error
		cancelBeforeRead   bool
		cancelAfterConsume bool
		wantErr            error
		wantDecoded        int
		wantConsumed       int
	}{
		{
			name:         "success",
			input:        "ab",
			wantDecoded:  3,
			wantConsumed: 2,
		},
		{
			name:      "create",
			input:     "ab",
			createErr: failure,
			wantErr:   failure,
		},
		{
			name:         "decode",
			input:        "ab",
			decodeErr:    failure,
			wantErr:      failure,
			wantDecoded:  2,
			wantConsumed: 1,
		},
		{
			name:         "read",
			input:        "a",
			readErr:      failure,
			wantErr:      failure,
			wantDecoded:  2,
			wantConsumed: 1,
		},
		{
			name:         "consumer",
			input:        "ab",
			consumerErr:  failure,
			wantErr:      failure,
			wantDecoded:  1,
			wantConsumed: 1,
		},
		{
			name:               "cancel",
			input:              "ab",
			cancelAfterConsume: true,
			wantErr:            context.Canceled,
			wantDecoded:        1,
			wantConsumed:       1,
		},
		{
			name:             "cancel_before_read",
			input:            "ab",
			cancelBeforeRead: true,
			wantErr:          context.Canceled,
		},
	}
	for _, signal := range []string{"logs", "metrics"} {
		for _, tt := range tests {
			t.Run(signal+"/"+tt.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var decoded, consumed int
				newDecoder := func(reader io.Reader) (streamTestDecoder, error) {
					return streamTestDecoder{decode: func() error {
						decoded++
						if decoded == 2 && tt.decodeErr != nil {
							return tt.decodeErr
						}
						var buf [1]byte
						_, err := reader.Read(buf[:])
						return err
					}}, tt.createErr
				}
				ext := streamTestExtension{
					logs:    func(r io.Reader) (encoding.LogsDecoder, error) { return newDecoder(r) },
					metrics: func(r io.Reader) (encoding.MetricsDecoder, error) { return newDecoder(r) },
				}
				consume := func() error {
					consumed++
					if tt.cancelAfterConsume {
						cancel()
					}
					return tt.consumerErr
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
				var input io.Reader = strings.NewReader(tt.input)
				if tt.readErr != nil {
					input = io.MultiReader(input, iotest.ErrReader(tt.readErr))
				}
				if tt.cancelBeforeRead {
					cancel()
				}
				err := r.receive(ctx, "object.txt", input)
				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, tt.wantDecoded, decoded)
				require.Equal(t, tt.wantConsumed, consumed)
			})
		}
	}
}

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
	err := r.receive(t.Context(), "object.txt", iotest.ErrReader(failure))
	require.ErrorIs(t, err, failure)
}

func TestReceiveCancellationAtEOF(t *testing.T) {
	tests := []struct {
		name             string
		cancelDuringRead bool
	}{
		{
			name: "decoder",
		},
		{
			name:             "drain",
			cancelDuringRead: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			ext := streamTestExtension{logs: func(io.Reader) (encoding.LogsDecoder, error) {
				return streamTestDecoder{decode: func() error {
					if !tt.cancelDuringRead {
						cancel()
					}
					return io.EOF
				}}, nil
			}}
			r := newStreamTestReceiver(t, &logsReceiver{}, ext)
			input := readFunc(func([]byte) (int, error) {
				cancel()
				return 0, io.EOF
			})
			require.ErrorIs(t, r.receive(ctx, "logs.txt", input), context.Canceled)
		})
	}
}

func TestReceiveBufferedSizeLimit(t *testing.T) {
	compressions := []struct {
		name     string
		suffix   string
		compress func([]byte) []byte
	}{
		{
			name:     "plain",
			compress: bytes.Clone,
		},
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
	signals := []struct {
		name      string
		processor func(func()) receiverProcessor
	}{
		{
			name: "logs",
			processor: func(consume func()) receiverProcessor {
				c, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
					consume()
					return nil
				})
				require.NoError(t, err)
				return &logsReceiver{consumer: c}
			},
		},
		{
			name: "metrics",
			processor: func(consume func()) receiverProcessor {
				c, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
					consume()
					return nil
				})
				require.NoError(t, err)
				return &metricsReceiver{consumer: c}
			},
		},
		{
			name: "traces",
			processor: func(consume func()) receiverProcessor {
				c, err := consumer.NewTraces(func(context.Context, ptrace.Traces) error {
					consume()
					return nil
				})
				require.NoError(t, err)
				return &traceReceiver{consumer: c}
			},
		},
	}
	content := []byte(strings.Repeat(" ", 4096) + "{}")
	tests := []struct {
		name         string
		limit        int64
		wantErr      error
		wantConsumed int
	}{
		{
			name:         "exact_limit",
			limit:        int64(len(content)),
			wantConsumed: 1,
		},
		{
			name:    "over_limit",
			limit:   int64(len(content) - 1),
			wantErr: errUnmarshalSizeLimit,
		},
		{
			name:    "large_expansion",
			limit:   16,
			wantErr: errUnmarshalSizeLimit,
		},
	}
	for _, signal := range signals {
		for _, compression := range compressions {
			for _, tt := range tests {
				t.Run(signal.name+"/"+compression.name+"/"+tt.name, func(t *testing.T) {
					var consumed int
					r := newStreamTestReceiver(t, signal.processor(func() { consumed++ }), nil)
					r.maxUnmarshalSize = tt.limit
					body := compression.compress(content)
					err := r.receive(t.Context(), "object.json"+compression.suffix, bytes.NewReader(body))
					require.ErrorIs(t, err, tt.wantErr)
					require.Equal(t, tt.wantConsumed, consumed)
				})
			}
		}
	}
}

func TestReceiveStreamingExceedsBufferedLimit(t *testing.T) {
	var consumed int
	c, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
		consumed += logs.LogRecordCount()
		return nil
	})
	require.NoError(t, err)
	r := newStreamTestReceiver(t, &logsReceiver{consumer: c}, newTextExtension(t))
	r.maxUnmarshalSize = 1
	require.NoError(t, r.receive(t.Context(), "logs.txt.gz", bytes.NewReader(gzipCompress([]byte("one\ntwo\n")))))
	require.Equal(t, 2, consumed)
}
