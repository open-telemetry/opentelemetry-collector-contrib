// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"

import (
	"bytes"
	"compress/gzip"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/config/configcompression"
)

type Manager interface {
	Upload(ctx context.Context, data []byte, opts *UploadOptions) error
}

type ManagerOpt func(Manager)

type UploadOptions struct {
	OverridePrefix string
}

type s3manager struct {
	bucket       string
	builder      *PartitionKeyBuilder
	uploader     *manager.Uploader
	storageClass s3types.StorageClass
	acl          s3types.ObjectCannedACL
}

var _ Manager = (*s3manager)(nil)

func NewS3Manager(bucket string, builder *PartitionKeyBuilder, service *s3.Client, storageClass s3types.StorageClass, opts ...ManagerOpt) Manager {
	manager := &s3manager{
		bucket:       bucket,
		builder:      builder,
		uploader:     manager.NewUploader(service),
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
	if sw.builder.Compression.IsCompressed() {
		encoding = string(sw.builder.Compression)
	}

	now := clock.Now(ctx)

	overridePrefix := ""
	if opts != nil {
		overridePrefix = opts.OverridePrefix
	}

	_, err = sw.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(sw.bucket),
		Key:             aws.String(sw.builder.Build(now, overridePrefix)),
		Body:            content,
		ContentEncoding: aws.String(encoding),
		StorageClass:    sw.storageClass,
		ACL:             sw.acl,
	})

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
