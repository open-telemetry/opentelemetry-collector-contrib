// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

const (
	// readBufferSize is the buffer size for sequential streaming reads.
	readBufferSize = 128 * 1024 // 128 KB
	// defaultChunkSize is the size of each cached chunk for S3 range reads.
	defaultChunkSize = 4 * 1024 * 1024 // 4 MB
	// defaultMaxCachedChunks is the maximum number of chunks held in the LRU cache.
	defaultMaxCachedChunks = 8
)

// s3API abstracts out the S3 APIs allowing mocking for tests
type s3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3ObjectReader provides access to an S3 object. It supports both sequential
// streaming via io.Reader (single GET) and random access via io.ReaderAt
// (chunk-cached range GETs). The concrete type also exposes a Size() int64
// method, discoverable via type assertion.
type S3ObjectReader interface {
	io.Reader
	io.ReaderAt
	io.Closer
}

// S3Service define services exposed for consumers
type S3Service interface {
	GetReader(ctx context.Context, bucketName, objectKey string, objectSize int64) (S3ObjectReader, error)
	ReadObject(ctx context.Context, bucketName, objectKey string) ([]byte, error)
	ListObjects(ctx context.Context, bucketName, continuationToken, prefix string) (*s3.ListObjectsV2Output, error)
	DeleteObject(ctx context.Context, bucketName, objectKey string) error
}

// S3Provider expose contract to get S3Service
type S3Provider interface {
	GetService(ctx context.Context) (S3Service, error)
}

// S3ServiceProvider provides S3Service instances.
type S3ServiceProvider struct{}

func (*S3ServiceProvider) GetService(ctx context.Context) (S3Service, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	return &s3ServiceClient{api: s3.NewFromConfig(cfg)}, nil
}

// s3ServiceClient implements the S3Service
type s3ServiceClient struct {
	api s3API
}

func (s *s3ServiceClient) ReadObject(ctx context.Context, bucketName, objectKey string) ([]byte, error) {
	params := s3.GetObjectInput{Bucket: &bucketName, Key: &objectKey}
	out, err := s.api.GetObject(ctx, &params)
	if err != nil {
		return nil, fmt.Errorf("unable to to download file from S3: %w", err)
	}

	defer func() {
		_ = out.Body.Close()
	}()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		// s3 body read error is marked for retrying
		return nil, consumererror.NewRetryableError(fmt.Errorf("error reading body from S3 file: %w", err))
	}

	return body, nil
}

func (s *s3ServiceClient) GetReader(ctx context.Context, bucketName, objectKey string, objectSize int64) (S3ObjectReader, error) {
	return &s3ObjectReader{
		ctx:       ctx,
		api:       s.api,
		bucket:    bucketName,
		key:       objectKey,
		size:      objectSize,
		chunkSize: defaultChunkSize,
		maxChunks: defaultMaxCachedChunks,
		cache:     make(map[int64][]byte),
	}, nil
}

// s3ObjectReader implements S3ObjectReader. Sequential reads via Read() use a
// single streaming GET (lazy-initialized on first call). Random access via
// ReadAt() uses chunk-cached range GETs.
type s3ObjectReader struct {
	ctx    context.Context
	api    s3API
	bucket string
	key    string
	size   int64

	// Streaming read state (lazy, initialized on first Read call).
	bodyCloser io.Closer     // original S3 response body, for Close()
	bodyReader *bufio.Reader // buffered wrapper, for Read()

	// Chunk cache for ReadAt.
	chunkSize int64
	maxChunks int

	mu         sync.Mutex
	cache      map[int64][]byte // chunkIndex -> data; nil after Close
	cacheOrder []int64          // LRU order, most recent at end
}

// Read implements io.Reader using a single streaming GET request, lazily
// initialized on the first call. This is the most efficient path for
// sequential consumption (e.g. gzipped or line-delimited content).
func (r *s3ObjectReader) Read(p []byte) (int, error) {
	br, err := r.getBodyReader()
	if err != nil {
		return 0, err
	}
	return br.Read(p)
}

// getBodyReader returns the lazily-initialized buffered reader, opening the
// S3 streaming GET on first call. mu is held only for the initialization check,
// not during the actual read.
func (r *s3ObjectReader) getBodyReader() (*bufio.Reader, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bodyReader == nil {
		out, err := r.api.GetObject(r.ctx, &s3.GetObjectInput{
			Bucket: &r.bucket,
			Key:    &r.key,
		})
		if err != nil {
			return nil, fmt.Errorf("S3 streaming GET for %s/%s failed: %w", r.bucket, r.key, err)
		}
		r.bodyCloser = out.Body
		r.bodyReader = bufio.NewReaderSize(out.Body, readBufferSize)
	}
	return r.bodyReader, nil
}

// Size returns the object size in bytes.
func (r *s3ObjectReader) Size() int64 { return r.size }

// ReadAt implements io.ReaderAt. It reads from the S3 object at the given
// offset using chunk-aligned range GETs with an LRU cache to minimize API calls.
func (r *s3ObjectReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.size {
		return 0, io.EOF
	}

	totalRead := 0
	for totalRead < len(p) && off < r.size {
		chunkIdx := off / r.chunkSize
		chunkData, err := r.getChunk(chunkIdx)
		if err != nil {
			if totalRead > 0 {
				return totalRead, err
			}
			return 0, err
		}

		offsetInChunk := off - chunkIdx*r.chunkSize
		n := copy(p[totalRead:], chunkData[offsetInChunk:])
		totalRead += n
		off += int64(n)
	}

	if totalRead < len(p) {
		return totalRead, io.EOF
	}
	return totalRead, nil
}

// Close releases the streaming body (if opened) and cached data.
func (r *s3ObjectReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = nil
	r.cacheOrder = nil
	if r.bodyCloser != nil {
		return r.bodyCloser.Close()
	}
	return nil
}

// getChunk returns the data for the given chunk index, fetching it via a range
// GET if not already cached.
func (r *s3ObjectReader) getChunk(idx int64) ([]byte, error) {
	r.mu.Lock()
	if r.cache == nil {
		r.mu.Unlock()
		return nil, io.ErrClosedPipe
	}
	if data, ok := r.cache[idx]; ok {
		r.touchLRU(idx)
		r.mu.Unlock()
		return data, nil
	}
	r.mu.Unlock()

	// Fetch the chunk outside the lock to allow concurrent reads to other chunks.
	start := idx * r.chunkSize
	end := start + r.chunkSize - 1
	if end >= r.size {
		end = r.size - 1
	}
	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)

	out, err := r.api.GetObject(r.ctx, &s3.GetObjectInput{
		Bucket: &r.bucket,
		Key:    &r.key,
		Range:  &rangeHeader,
	})
	if err != nil {
		return nil, fmt.Errorf("S3 range GET for chunk %d of %s/%s failed: %w", idx, r.bucket, r.key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("reading S3 range response for chunk %d: %w", idx, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cache == nil {
		return nil, io.ErrClosedPipe
	}

	// Another goroutine may have fetched the same chunk concurrently.
	if existing, ok := r.cache[idx]; ok {
		r.touchLRU(idx)
		return existing, nil
	}

	// Evict if at capacity.
	for len(r.cache) >= r.maxChunks && len(r.cacheOrder) > 0 {
		evict := r.cacheOrder[0]
		r.cacheOrder = r.cacheOrder[1:]
		delete(r.cache, evict)
	}

	r.cache[idx] = data
	r.cacheOrder = append(r.cacheOrder, idx)
	return data, nil
}

// touchLRU moves idx to the end of the LRU order. Must be called with mu held.
func (r *s3ObjectReader) touchLRU(idx int64) {
	for i, v := range r.cacheOrder {
		if v == idx {
			r.cacheOrder = append(r.cacheOrder[:i], r.cacheOrder[i+1:]...)
			break
		}
	}
	r.cacheOrder = append(r.cacheOrder, idx)
}

func (s *s3ServiceClient) ListObjects(ctx context.Context, bucketName, continuationToken, prefix string) (*s3.ListObjectsV2Output, error) {
	input := s3.ListObjectsV2Input{
		Bucket: &bucketName,
	}

	if continuationToken != "" {
		input.ContinuationToken = &continuationToken
	}

	if prefix != "" {
		input.Prefix = &prefix
	}
	return s.api.ListObjectsV2(ctx, &input)
}

func (s *s3ServiceClient) DeleteObject(ctx context.Context, bucketName, objectKey string) error {
	input := s3.DeleteObjectInput{Bucket: &bucketName, Key: &objectKey}

	_, err := s.api.DeleteObject(ctx, &input)
	if err != nil {
		return fmt.Errorf("unable to to delete file from S3: %w", err)
	}

	return nil
}
