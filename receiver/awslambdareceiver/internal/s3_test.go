// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

func TestS3ServiceProvider(t *testing.T) {
	provider := S3ServiceProvider{}

	service, err := provider.GetService(t.Context())
	require.NoError(t, err)
	require.NotNil(t, service)
}

func TestS3ServiceProviderWithTracerProvider(t *testing.T) {
	tp := nooptrace.NewTracerProvider()
	provider := S3ServiceProvider{TracerProvider: tp}

	service, err := provider.GetService(t.Context())
	require.NoError(t, err)
	require.NotNil(t, service)
}

func TestS3ServiceReadObject(t *testing.T) {
	gCtr := gomock.NewController(t)

	t.Run("Successful read", func(t *testing.T) {
		bucketName := "bucket"
		objectKey := "object"

		reader := bytes.NewReader([]byte("content"))
		readerCloser := io.NopCloser(reader)

		input := &s3.GetObjectInput{Bucket: &bucketName, Key: &objectKey}
		output := &s3.GetObjectOutput{Body: readerCloser}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().GetObject(gomock.Any(), gomock.Eq(input), gomock.Any()).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}

		object, err := client.ReadObject(t.Context(), bucketName, objectKey)
		require.NoError(t, err)

		require.Equal(t, "content", string(object))
	})

	t.Run("Body read errors returns a retryable error", func(t *testing.T) {
		bucketName := "bucket"
		objectKey := "object"

		input := &s3.GetObjectInput{Bucket: &bucketName, Key: &objectKey}
		output := &s3.GetObjectOutput{Body: mockReaderCloser{err: errors.New("some network error")}}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().GetObject(gomock.Any(), gomock.Eq(input), gomock.Any()).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		_, err := client.ReadObject(t.Context(), bucketName, objectKey)
		require.Error(t, err)
		var consumerErr *consumererror.Error
		require.ErrorAs(t, err, &consumerErr)
	})
}

func TestS3ServiceListObject(t *testing.T) {
	gCtr := gomock.NewController(t)

	t.Run("Successful listing - bucket name", func(t *testing.T) {
		bucketName := "bucket"

		input := &s3.ListObjectsV2Input{Bucket: &bucketName}
		output := &s3.ListObjectsV2Output{Contents: []types.Object{}}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().ListObjectsV2(gomock.Any(), gomock.Eq(input)).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		out, err := client.ListObjects(t.Context(), bucketName, "", "")
		require.NoError(t, err)
		require.Equal(t, output, out)
	})

	t.Run("Successful listing - bucket name with continuation token", func(t *testing.T) {
		bucketName := "bucket"
		token := "token"

		input := &s3.ListObjectsV2Input{Bucket: &bucketName, ContinuationToken: &token}
		output := &s3.ListObjectsV2Output{Contents: []types.Object{}}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().ListObjectsV2(gomock.Any(), gomock.Eq(input)).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		out, err := client.ListObjects(t.Context(), bucketName, token, "")
		require.NoError(t, err)
		require.Equal(t, output, out)
	})
}

func TestS3ServiceDeleteObject(t *testing.T) {
	gCtr := gomock.NewController(t)

	t.Run("Successful delete", func(t *testing.T) {
		bucketName := "bucket"
		objectKey := "object"

		input := &s3.DeleteObjectInput{Bucket: &bucketName, Key: &objectKey}
		output := &s3.DeleteObjectOutput{}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().DeleteObject(gomock.Any(), gomock.Eq(input)).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		err := client.DeleteObject(t.Context(), bucketName, objectKey)
		require.NoError(t, err)
	})
}

// mockGetObjectForData returns a DoAndReturn function that serves both full-object
// and range GETs from data.
func mockGetObjectForData(t *testing.T, data []byte) func(any, *s3.GetObjectInput, ...any) (*s3.GetObjectOutput, error) {
	t.Helper()
	return func(_ any, input *s3.GetObjectInput, _ ...any) (*s3.GetObjectOutput, error) {
		if input.Range == nil {
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
		}
		var start, end int64
		_, err := fmt.Sscanf(*input.Range, "bytes=%d-%d", &start, &end)
		require.NoError(t, err)
		body := io.NopCloser(io.NewSectionReader(NewBytesS3ObjectReader(data), start, end-start+1))
		return &s3.GetObjectOutput{Body: body}, nil
	}
}

func newTestReader(api s3API, data []byte, chunkSize int64, maxChunks int) *s3ObjectReader {
	return &s3ObjectReader{
		ctx:       context.Background(),
		api:       api,
		bucket:    "test-bucket",
		key:       "test-key",
		size:      int64(len(data)),
		chunkSize: chunkSize,
		maxChunks: maxChunks,
		cache:     make(map[int64][]byte),
	}
}

func TestS3ObjectReader_ReadAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("Hello, World! This is test data for ReadAt.")

	reader := newTestReader(api, data, 10, 8)
	defer reader.Close()

	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(mockGetObjectForData(t, data)).AnyTimes()

	// Read from the middle of the data.
	buf := make([]byte, 5)
	n, err := reader.ReadAt(buf, 7)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "World", string(buf))

	// Read spanning two chunks.
	buf = make([]byte, 8)
	n, err = reader.ReadAt(buf, 8)
	require.NoError(t, err)
	assert.Equal(t, 8, n)
	assert.Equal(t, "orld! Th", string(buf))
}

func TestS3ObjectReader_ReadAt_EOF(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("short")

	reader := newTestReader(api, data, 10, 4)
	defer reader.Close()

	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(mockGetObjectForData(t, data)).AnyTimes()

	// Read past end returns io.EOF.
	buf := make([]byte, 10)
	n, err := reader.ReadAt(buf, 0)
	assert.Equal(t, 5, n)
	assert.ErrorIs(t, err, io.EOF)

	// Read at exactly size returns io.EOF with 0 bytes.
	n, err = reader.ReadAt(buf, 5)
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}

func TestS3ObjectReader_ChunkCaching(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("0123456789ABCDEF")

	reader := newTestReader(api, data, 8, 4)
	defer reader.Close()

	// Expect exactly 1 call for chunk 0 — the second read should hit the cache.
	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(mockGetObjectForData(t, data)).Times(1)

	buf := make([]byte, 4)
	_, err := reader.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, "0123", string(buf))

	// Second read within the same chunk — no new API call.
	_, err = reader.ReadAt(buf, 4)
	require.NoError(t, err)
	assert.Equal(t, "4567", string(buf))
}

func TestS3ObjectReader_LRUEviction(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("0123456789ABCDEFGHIJKLMNOPQRst")

	reader := newTestReader(api, data, 10, 2)
	defer reader.Close()

	callCount := 0
	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx any, input *s3.GetObjectInput, opts ...any) (*s3.GetObjectOutput, error) {
			callCount++
			return mockGetObjectForData(t, data)(ctx, input, opts)
		}).AnyTimes()

	buf := make([]byte, 1)

	// Load chunk 0.
	_, err := reader.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Load chunk 1 — cache now has [0, 1].
	_, err = reader.ReadAt(buf, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)

	// Load chunk 2 — evicts chunk 0 (LRU), cache now has [1, 2].
	_, err = reader.ReadAt(buf, 20)
	require.NoError(t, err)
	assert.Equal(t, 3, callCount)

	// Re-read chunk 1 — still cached.
	_, err = reader.ReadAt(buf, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, callCount)

	// Re-read chunk 0 — was evicted, must fetch again.
	_, err = reader.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, 4, callCount)
}

func TestS3ObjectReader_Close(t *testing.T) {
	reader := &s3ObjectReader{
		size:      100,
		chunkSize: 10,
		maxChunks: 4,
		cache:     map[int64][]byte{0: []byte("data")},
	}

	err := reader.Close()
	require.NoError(t, err)
	assert.Nil(t, reader.cache)
	assert.Nil(t, reader.cacheOrder)
}

func TestS3ObjectReader_Read(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("streaming content here")

	reader := newTestReader(api, data, 10, 4)
	defer reader.Close()

	// Expect a single full-object GET (no Range header).
	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, input *s3.GetObjectInput, _ ...any) (*s3.GetObjectOutput, error) {
			assert.Nil(t, input.Range, "streaming Read should not use Range header")
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
		}).Times(1)

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestS3ObjectReader_Size(t *testing.T) {
	reader := &s3ObjectReader{size: 12345}
	assert.Equal(t, int64(12345), reader.Size())
}

func TestS3ServiceGetReader(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	client := s3ServiceClient{api: api}

	reader, err := client.GetReader(t.Context(), "bucket", "key", 42)
	require.NoError(t, err)

	objReader, ok := reader.(*s3ObjectReader)
	require.True(t, ok)
	assert.Equal(t, int64(42), objReader.size)
	assert.Equal(t, "bucket", objReader.bucket)
	assert.Equal(t, "key", objReader.key)
	assert.NotNil(t, objReader.ctx)
}

func TestS3ObjectReader_Close_DuringRead_Init(t *testing.T) {
	// Verifies that Close() called while Read()'s lazy GetObject is in-flight
	// produces no panic, no deadlock, and leaves the reader closed.
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("close during read init test")

	inGET := make(chan struct{})
	blockGET := make(chan struct{})
	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, input *s3.GetObjectInput, _ ...any) (*s3.GetObjectOutput, error) {
			assert.Nil(t, input.Range, "Read should not use a Range header")
			close(inGET)
			<-blockGET
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
		}).Times(1)

	reader := newTestReader(api, data, 10, 4)

	var wg sync.WaitGroup
	wg.Go(func() {
		buf := make([]byte, 4)
		reader.Read(buf) //nolint:errcheck
	})

	<-inGET         // GetObject is in-flight; mu is held by getBodyReader
	close(blockGET) // unblock GetObject; getBodyReader will release mu shortly
	// Close() may race with mu being released — both orderings are valid.
	require.NoError(t, reader.Close())
	wg.Wait()

	assert.Nil(t, reader.cache)
}

func TestS3ObjectReader_ReadAt_AfterClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("some data")

	reader := newTestReader(api, data, 10, 4)
	require.NoError(t, reader.Close())

	buf := make([]byte, 4)
	n, err := reader.ReadAt(buf, 0)
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestS3ObjectReader_ContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("cancel test data")

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // pre-cancel so every GetObject sees a done context

	reader := &s3ObjectReader{
		ctx:       ctx,
		api:       api,
		bucket:    "test-bucket",
		key:       "test-key",
		size:      int64(len(data)),
		chunkSize: 10,
		maxChunks: 4,
		cache:     make(map[int64][]byte),
	}
	defer reader.Close()

	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(callCtx context.Context, _ *s3.GetObjectInput, _ ...any) (*s3.GetObjectOutput, error) {
			return nil, callCtx.Err()
		}).AnyTimes()

	buf := make([]byte, 4)
	_, err := reader.Read(buf)
	assert.ErrorIs(t, err, context.Canceled)

	_, err = reader.ReadAt(buf, 0)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestS3ObjectReader_Close_DuringInFlightReadAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := NewMocks3API(ctrl)
	data := []byte("0123456789") // one 10-byte chunk

	reader := newTestReader(api, data, 10, 4)

	inGET := make(chan struct{}) // closed when GetObject is entered (lock released)
	blockGET := make(chan struct{})
	api.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, _ *s3.GetObjectInput, _ ...any) (*s3.GetObjectOutput, error) {
			close(inGET) // signal: lock is released, range GET is in-flight
			<-blockGET
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
		}).Times(1)

	errc := make(chan error, 1)
	go func() {
		buf := make([]byte, 5)
		_, err := reader.ReadAt(buf, 0)
		errc <- err
	}()

	<-inGET                            // wait until getChunk has released mu and is inside GetObject
	require.NoError(t, reader.Close()) // Close acquires mu (now free) and marks closed
	close(blockGET)                    // let GetObject return; getChunk will see r.closed and return ErrClosedPipe

	err := <-errc
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

type mockReaderCloser struct {
	err error
}

func (m mockReaderCloser) Close() error {
	return m.err
}

func (m mockReaderCloser) Read(p []byte) (n int, err error) {
	return len(p), m.err
}

func (m mockReaderCloser) Write(p []byte) (n int, err error) {
	return len(p), m.err
}
