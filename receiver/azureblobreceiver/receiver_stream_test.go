// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

func TestStreamLogs(t *testing.T) {
	var input strings.Builder
	for i := range 2501 {
		fmt.Fprintf(&input, "{\"RayID\":\"%d\"}\n", i)
	}
	plain := []byte(strings.TrimSuffix(input.String(), "\n"))
	compressed := gzipBytes(t, plain)

	for _, tt := range []struct {
		name        string
		compression string
		data        []byte
	}{
		{name: "plain", data: plain},
		{name: "gzip", compression: CompressionGzip, data: compressed},
		{name: "auto plain", compression: CompressionAuto, data: plain},
		{name: "auto gzip", compression: CompressionAuto, data: compressed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			r := newTextReceiver(t, "polling", tt.compression, newMockBlobClient(), sink)
			require.NoError(t, r.consumeLogs(t.Context(), bytes.NewReader(tt.data)))

			batches := sink.AllLogs()
			require.Len(t, batches, 3)
			assert.Equal(t, 1000, batches[0].LogRecordCount())
			assert.Equal(t, 1000, batches[1].LogRecordCount())
			assert.Equal(t, 501, batches[2].LogRecordCount())
			var bodies []string
			for _, batch := range batches {
				for _, resource := range batch.ResourceLogs().All() {
					for _, scope := range resource.ScopeLogs().All() {
						for _, record := range scope.LogRecords().All() {
							bodies = append(bodies, record.Body().Str())
						}
					}
				}
			}
			assert.Equal(t, strings.Split(string(plain), "\n"), bodies)
		})
	}
}

func TestStreamLogsConsumesBeforeReadingRemainder(t *testing.T) {
	for _, compression := range []string{CompressionNone, CompressionGzip, CompressionAuto} {
		t.Run(compression, func(t *testing.T) {
			prefix := []byte(strings.Repeat("{\"RayID\":\"first\"}\n", 1000))
			tail := []byte("{\"RayID\":\"last\"}\n")
			data := append(bytes.Clone(prefix), tail...)
			boundary := len(prefix)
			if compression != CompressionNone {
				var buf bytes.Buffer
				writer := gzip.NewWriter(&buf)
				_, err := writer.Write(prefix)
				require.NoError(t, err)
				require.NoError(t, writer.Flush())
				boundary = buf.Len()
				_, err = writer.Write(tail)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				data = buf.Bytes()
			}

			input := &gatedReader{data: data, boundary: boundary}
			var batchSizes []int
			next, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
				batchSizes = append(batchSizes, logs.LogRecordCount())
				input.consumed = true
				return nil
			})
			require.NoError(t, err)
			r := newTextReceiver(t, "polling", compression, newMockBlobClient(), next)

			require.NoError(t, r.consumeLogs(t.Context(), input))
			assert.Equal(t, []int{1000, 1}, batchSizes)
		})
	}
}

func TestStreamBlobCompletion(t *testing.T) {
	plain := []byte(strings.Repeat("{\"RayID\":\"test\"}\n", 2501))
	compressed := gzipBytes(t, plain)
	corrupt := bytes.Clone(compressed)
	corrupt[len(corrupt)-8] ^= 0xff
	readErr := errors.New("download interrupted")
	consumeErr := errors.New("downstream rejected batch")

	for _, mode := range testModes {
		for _, tt := range []struct {
			name        string
			reader      func() io.Reader
			consumeErr  error
			cancelAfter bool
			wantErr     error
			wantBatches int
		}{
			{name: "success", reader: func() io.Reader { return bytes.NewReader(compressed) }, wantBatches: 3},
			{name: "checksum", reader: func() io.Reader { return bytes.NewReader(corrupt) }, wantErr: gzip.ErrChecksum, wantBatches: 2},
			{name: "truncated", reader: func() io.Reader { return bytes.NewReader(compressed[:len(compressed)-8]) }, wantErr: io.ErrUnexpectedEOF, wantBatches: 2},
			{name: "read failure", reader: func() io.Reader {
				return io.MultiReader(bytes.NewReader(compressed[:len(compressed)-8]), failingReader{err: readErr})
			}, wantErr: readErr, wantBatches: 2},
			{name: "consumer failure", reader: func() io.Reader { return bytes.NewReader(compressed) }, consumeErr: consumeErr, wantErr: consumeErr, wantBatches: 2},
			{name: "cancellation", reader: func() io.Reader { return bytes.NewReader(compressed) }, cancelAfter: true, wantErr: context.Canceled, wantBatches: 1},
			{name: "bad gzip header", reader: func() io.Reader { return strings.NewReader("invalid gzip payload") }, wantErr: gzip.ErrHeader},
		} {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				input := &trackedReadCloser{Reader: tt.reader()}
				client := &mockBlobClient{}
				client.On("readBlob", mock.Anything, logsContainerName, "logs-1").Return(input, nil).Once()
				batches := 0
				next, err := consumer.NewLogs(func(_ context.Context, _ plog.Logs) error {
					batches++
					if tt.cancelAfter {
						cancel()
					}
					if batches == 2 {
						return tt.consumeErr
					}
					return nil
				})
				require.NoError(t, err)
				if tt.wantErr == nil {
					client.On("deleteBlob", mock.Anything, logsContainerName, "logs-1").Run(func(mock.Arguments) {
						assert.Equal(t, tt.wantBatches, batches)
					}).Return(nil).Once()
				}
				r := newTextReceiver(t, mode, CompressionGzip, client, next)

				switch handler := r.blobEventHandler.(type) {
				case *blobEventHandler:
					handler.processBlob(ctx, logsContainerName, "logs-1", r.consumeLogs)
				case *eventHubEventHandler:
					err = handler.newMessageHandler(ctx, getEventHubEvent(logEventData))
					if tt.wantErr != nil {
						require.ErrorIs(t, err, tt.wantErr)
					} else {
						require.NoError(t, err)
					}
				}
				assert.Equal(t, tt.wantBatches, batches)
				assert.Equal(t, 1, input.closes)
				client.AssertExpectations(t)
				if tt.wantErr != nil {
					client.AssertNotCalled(t, "deleteBlob", mock.Anything, mock.Anything, mock.Anything)
				}
			})
		}
	}
}

func TestDecompressAuto(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("x"), []byte("plain"), {0x1f}, gzipBytes(t, []byte("compressed"))} {
		t.Run(fmt.Sprintf("%x", data), func(t *testing.T) {
			r := &blobReceiver{compression: CompressionAuto}
			reader, err := r.decompress(bytes.NewReader(data))
			require.NoError(t, err)
			defer reader.Close()
			decoded, err := io.ReadAll(reader)
			require.NoError(t, err)
			if bytes.HasPrefix(data, gzipMagic) {
				assert.Equal(t, "compressed", string(decoded))
			} else {
				assert.Equal(t, string(data), string(decoded))
			}
		})
	}
	readErr := errors.New("header read failed")
	r := &blobReceiver{compression: CompressionAuto}
	_, err := r.decompress(failingReader{err: readErr})
	require.ErrorIs(t, err, readErr)
}

func TestLogsDecoderOnlyExtension(t *testing.T) {
	decoder := xstreamencoding.NewLogsUnmarshalerDecoderFactory(&plog.JSONUnmarshaler{})
	host := &hostWithExtensions{extensions: map[component.ID]component.Component{
		component.MustNewID("decoder_only"): &decoderOnlyExtension{LogsDecoderFactory: decoder},
	}}
	r, err := getBlobReceiverWithHost(t, "polling", "decoder_only", EncodingOTLPJSON, host)
	require.NoError(t, err)
	sink := &consumertest.LogsSink{}
	r.(*blobReceiver).setNextLogsConsumer(sink)
	require.NoError(t, r.(*blobReceiver).consumeLogs(t.Context(), bytes.NewReader(logsJSON)))
	assert.Equal(t, 1, sink.LogRecordCount())
}

func TestStreamLogsDecoderErrors(t *testing.T) {
	decodeErr := errors.New("decoder failed")
	for _, constructionFailure := range []bool{false, true} {
		t.Run(fmt.Sprintf("construction=%v", constructionFailure), func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			r := newTextReceiver(t, "polling", CompressionNone, newMockBlobClient(), sink)
			r.logsDecoder = logsDecoderFactoryFunc(func(io.Reader, ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
				if constructionFailure {
					return nil, decodeErr
				}
				return xstreamencoding.NewLogsDecoderAdapter(func() (plog.Logs, error) {
					return plog.Logs{}, decodeErr
				}, func() int64 { return 0 }), nil
			})
			require.ErrorIs(t, r.consumeLogs(t.Context(), strings.NewReader("input")), decodeErr)
			assert.Zero(t, sink.LogRecordCount())
		})
	}
}

func TestStreamLogsValidatesTrailerAfterDecoderEOF(t *testing.T) {
	data := gzipBytes(t, []byte("payload"))
	data[len(data)-8] ^= 0xff
	r := newTextReceiver(t, "polling", CompressionGzip, newMockBlobClient(), consumertest.NewNop())
	r.logsDecoder = logsDecoderFactoryFunc(func(io.Reader, ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
		return xstreamencoding.NewLogsDecoderAdapter(func() (plog.Logs, error) {
			return plog.Logs{}, io.EOF
		}, func() int64 { return 0 }), nil
	})
	require.ErrorIs(t, r.consumeLogs(t.Context(), bytes.NewReader(data)), gzip.ErrChecksum)
}

type logsDecoderFactoryFunc func(io.Reader, ...encoding.DecoderOption) (encoding.LogsDecoder, error)

func (f logsDecoderFactoryFunc) NewLogsDecoder(r io.Reader, opts ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return f(r, opts...)
}

func newTextReceiver(t *testing.T, mode, compression string, client blobClient, next consumer.Logs) *blobReceiver {
	t.Helper()
	factory := textencodingextension.NewFactory()
	ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig())
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(t.Context())) })
	host := &hostWithExtensions{extensions: map[component.ID]component.Component{component.NewID(factory.Type()): ext}}
	var handler eventHandler
	if mode == "polling" {
		handler = newBlobEventHandler(logsContainerName, tracesContainerName, client, zaptest.NewLogger(t))
	} else {
		handler = newEventHubEventHandler(eventHubString, logsContainerName, tracesContainerName, client, zaptest.NewLogger(t))
	}
	r, err := newReceiver(receivertest.NewNopSettings(metadata.Type), handler, factory.Type().String(), EncodingOTLPJSON, compression)
	require.NoError(t, err)
	receiver := r.(*blobReceiver)
	require.NoError(t, receiver.resolveEncodings(host))
	receiver.setNextLogsConsumer(next)
	return receiver
}

type trackedReadCloser struct {
	io.Reader
	closes int
}

func (r *trackedReadCloser) Close() error {
	r.closes++
	return nil
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

// Reading the next chunk is only allowed after the first batch has been consumed.
type gatedReader struct {
	data     []byte
	boundary int
	offset   int
	consumed bool
}

func (r *gatedReader) Read(p []byte) (int, error) {
	if r.offset == len(r.data) {
		return 0, io.EOF
	}
	end := len(r.data)
	if !r.consumed {
		if r.offset == r.boundary {
			return 0, errors.New("read past first batch before consuming it")
		}
		end = r.boundary
	}
	n := copy(p, r.data[r.offset:end])
	r.offset += n
	return n, nil
}

type decoderOnlyExtension struct {
	nonUnmarshalerExtension
	encoding.LogsDecoderFactory
}
