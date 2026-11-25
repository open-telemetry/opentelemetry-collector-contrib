// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfermanagertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/tilinna/clock"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"
)

type Manager interface {
	Upload(ctx context.Context, data []byte, opts *UploadOptions) error
}

type ManagerOpt func(Manager)

type UploadOptions struct {
	OverrideBucket string
	OverridePrefix string
}

type s3manager struct {
	logger       *zap.Logger
	bucket       string
	builder      *PartitionKeyBuilder
	uploader     *transfermanager.Client
	storageClass s3types.StorageClass
	acl          s3types.ObjectCannedACL
	writer       xcompression.Writer
}

var _ Manager = (*s3manager)(nil)

func WithACL(acl s3types.ObjectCannedACL) ManagerOpt {
	return func(m Manager) {
		s3m, ok := m.(*s3manager)
		if !ok {
			return
		}
		s3m.acl = acl
	}
}

func WithCompressionWriter(w xcompression.Writer) ManagerOpt {
	return func(m Manager) {
		s3m, ok := m.(*s3manager)
		if !ok {
			return
		}
		s3m.writer = w
	}
}

func NewS3Manager(logger *zap.Logger, bucket string, builder *PartitionKeyBuilder, service *s3.Client, storageClass s3types.StorageClass, opts ...ManagerOpt) Manager {
	manager := &s3manager{
		logger:  logger,
		bucket:  bucket,
		builder: builder,
		uploader: transfermanager.New(service, func(o *transfermanager.Options) {
			// transfermanager.New ignores the S3 client checksum setting and always
			// adds a CRC32 trailer, which some S3 compatible stores reject. So we
			// pass the client's own setting here so it is honored.
			o.RequestChecksumCalculation = service.Options().RequestChecksumCalculation
		}),
		storageClass: storageClass,
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

	if encoding := sw.builder.contentEncoding(); encoding != "" {
		uploadInput.ContentEncoding = aws.String(encoding)
	}

	sw.logger.Debug("uploading object", zap.String("bucket", overrideBucket), zap.String("key", key))
	_, err = sw.uploader.UploadObject(ctx, uploadInput)
	return err
}

func (sw *s3manager) contentBuffer(raw []byte) (*bytes.Buffer, error) {
	if sw.writer == nil {
		return bytes.NewBuffer(raw), nil
	}
	content := bytes.NewBuffer(nil)
	if _, err := sw.writer.CompressBytes(content, raw); err != nil {
		return nil, err
	}
	return content, nil
}
