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
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/config/configcompression"
)

type Manager interface {
	Upload(ctx context.Context, data []byte) error
}

type s3manager struct {
	bucket   string
	builder  *PartitionKeyBuilder
	uploader *manager.Uploader
}

var _ Manager = (*s3manager)(nil)

func NewS3Manager(bucket string, builder *PartitionKeyBuilder, service *s3.Client) Manager {
	return &s3manager{
		bucket:   bucket,
		builder:  builder,
		uploader: manager.NewUploader(service),
	}
}

func (sw *s3manager) Upload(ctx context.Context, data []byte) error {
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

	_, err = sw.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(sw.bucket),
		Key:             aws.String(sw.builder.Build(now)),
		Body:            content,
		ContentEncoding: aws.String(encoding),
	})

	return err
}

func (sw *s3manager) contentBuffer(raw []byte) (*bytes.Buffer, error) {
	//nolint:gocritic // Leaving this as a switch statement to make it easier to add more later compressions
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
	}
	return bytes.NewBuffer(raw), nil
}
