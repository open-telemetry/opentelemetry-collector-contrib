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
	bucket       string
	builder      *PartitionKeyBuilder
	uploader     *transfermanager.Client
	storageClass s3types.StorageClass
	acl          s3types.ObjectCannedACL
}

var _ Manager = (*s3manager)(nil)

func NewS3Manager(bucket string, builder *PartitionKeyBuilder, service *s3.Client, storageClass s3types.StorageClass, opts ...ManagerOpt) Manager {
	manager := &s3manager{
		bucket:       bucket,
		builder:      builder,
		uploader:     transfermanager.New(service),
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

	uploadInput := &transfermanager.UploadObjectInput{
		Bucket:       aws.String(overrideBucket),
		Key:          aws.String(sw.builder.Build(now, overridePrefix)),
		Body:         content,
		StorageClass: transfermanagertypes.StorageClass(sw.storageClass),
		ACL:          transfermanagertypes.ObjectCannedACL(sw.acl),
	}

	// Only set ContentEncoding if we have a non-empty encoding value
	if encoding != "" {
		uploadInput.ContentEncoding = aws.String(encoding)
	}

	_, err = sw.uploader.UploadObject(ctx, uploadInput)
	return err
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
