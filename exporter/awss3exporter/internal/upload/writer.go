// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"bytes"
	"compress/gzip"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfermanagertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/klauspost/compress/zstd"
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.uber.org/zap"
)

type Manager interface {
	Upload(ctx context.Context, data []byte, opts *UploadOptions) error
}

type ManagerOpt func(Manager)

type UploadOptions struct {
	OverrideBucket string
	OverridePrefix string
}

// NotifyEvent is the minimum tuple published to EventNotifier after each
// successful S3 upload. It is declared here — rather than imported from the
// notify sub-package — to keep upload free of any dependency on notify and
// thereby prevent an import cycle through the exporter root package.
type NotifyEvent struct {
	// Bucket is the S3 bucket the object was written to.
	Bucket string
	// Key is the raw object key. Consumers are responsible for any
	// encoding required by their transport.
	Key string
	// Size is the number of bytes written to S3 (post-compression).
	Size int64
}

// EventNotifier is the subset of the notify.Notifier contract the upload
// manager needs. The exporter wires a thin adapter from notify.Notifier to
// this interface.
type EventNotifier interface {
	Enqueue(ctx context.Context, e NotifyEvent) bool
}

// noopEventNotifier is the default EventNotifier assigned when WithNotifier
// is not applied. It exists so the upload manager can call
// sw.notifier.Enqueue unconditionally.
type noopEventNotifier struct{}

func (noopEventNotifier) Enqueue(_ context.Context, _ NotifyEvent) bool { return false }

type s3manager struct {
	logger       *zap.Logger
	bucket       string
	builder      *PartitionKeyBuilder
	uploader     *transfermanager.Client
	storageClass s3types.StorageClass
	acl          s3types.ObjectCannedACL
	notifier     EventNotifier
}

var _ Manager = (*s3manager)(nil)

func NewS3Manager(logger *zap.Logger, bucket string, builder *PartitionKeyBuilder, service *s3.Client, storageClass s3types.StorageClass, opts ...ManagerOpt) Manager {
	manager := &s3manager{
		logger:       logger,
		bucket:       bucket,
		builder:      builder,
		uploader:     transfermanager.New(service),
		storageClass: storageClass,
		notifier:     noopEventNotifier{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(manager)
		}
	}

	return manager
}

func (sw *s3manager) Upload(ctx context.Context, data []byte, opts *UploadOptions) error {
	if len(data) == 0 {
		return nil
	}

	content, err := sw.contentBuffer(data)
	if err != nil {
		return err
	}
	// Capture the post-compression byte count before the buffer is consumed
	// by UploadObject, for the notify event.
	uploadSize := int64(content.Len())

	encoding := ""
	// Only use ContentEncoding for non-archive formats
	// Archive formats store files compressed permanently (like .tar.gz)
	// while ContentEncoding is for HTTP transfer compression
	if sw.builder.Compression.IsCompressed() && !sw.builder.IsCompressed {
		encoding = string(sw.builder.Compression)
	}

	now := clock.Now(ctx)

	overridePrefix := ""
	overrideBucket := sw.bucket
	if opts != nil {
		overridePrefix = opts.OverridePrefix
		if opts.OverrideBucket != "" {
			overrideBucket = opts.OverrideBucket
		}
	}

	key := sw.builder.Build(now, overridePrefix)
	uploadInput := &transfermanager.UploadObjectInput{
		Bucket:       aws.String(overrideBucket),
		Key:          aws.String(key),
		Body:         content,
		StorageClass: transfermanagertypes.StorageClass(sw.storageClass),
		ACL:          transfermanagertypes.ObjectCannedACL(sw.acl),
	}

	// Only set ContentEncoding if we have a non-empty encoding value
	if encoding != "" {
		uploadInput.ContentEncoding = aws.String(encoding)
	}

	sw.logger.Debug("uploading object", zap.String("bucket", overrideBucket), zap.String("key", key))
	if _, err = sw.uploader.UploadObject(ctx, uploadInput); err != nil {
		return err
	}

	// The notifier's Enqueue is non-blocking and drops under back-pressure;
	// its return value is intentionally ignored so a slow webhook cannot
	// degrade the S3 upload path.
	sw.notifier.Enqueue(ctx, NotifyEvent{
		Bucket: overrideBucket,
		Key:    key,
		Size:   uploadSize,
	})
	return nil
}

func (sw *s3manager) contentBuffer(raw []byte) (*bytes.Buffer, error) {
	switch sw.builder.Compression {
	case configcompression.TypeGzip:
		content := bytes.NewBuffer(nil)

		zipper := gzip.NewWriter(content)
		if _, err := zipper.Write(raw); err != nil {
			return nil, err
		}
		if err := zipper.Close(); err != nil {
			return nil, err
		}

		return content, nil
	case configcompression.TypeZstd:
		content := bytes.NewBuffer(nil)
		zipper, err := zstd.NewWriter(content)
		if err != nil {
			return nil, err
		}
		_, err = zipper.Write(raw)
		if err != nil {
			return nil, err
		}
		err = zipper.Close()
		if err != nil {
			return nil, err
		}

		return content, nil
	default:
		return bytes.NewBuffer(raw), nil
	}
}

func WithACL(acl s3types.ObjectCannedACL) func(Manager) {
	return func(m Manager) {
		s3m, ok := m.(*s3manager)
		if !ok {
			return
		}
		s3m.acl = acl
	}
}

// WithNotifier installs the EventNotifier invoked after each successful
// UploadObject call. Passing nil leaves the default no-op notifier in place.
func WithNotifier(n EventNotifier) ManagerOpt {
	return func(m Manager) {
		s3m, ok := m.(*s3manager)
		if !ok {
			return
		}
		if n == nil {
			s3m.notifier = noopEventNotifier{}
			return
		}
		s3m.notifier = n
	}
}
