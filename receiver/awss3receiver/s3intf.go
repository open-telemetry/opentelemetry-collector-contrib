// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var downloadManager *manager.Downloader //nolint:golint,unused

type ListObjectsV2Pager interface {
	HasMorePages() bool
	NextPage(context.Context, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type ListObjectsAPI interface {
	NewListObjectsV2Paginator(params *s3.ListObjectsV2Input) ListObjectsV2Pager
}

type GetObjectAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type s3ListObjectsAPIImpl struct {
	client *s3.Client
}

func newS3Client(ctx context.Context, cfg S3DownloaderConfig) (ListObjectsAPI, GetObjectAPI, error) {
	optionsFuncs := make([]func(*config.LoadOptions) error, 0)
	if cfg.Region != "" {
		optionsFuncs = append(optionsFuncs, config.WithRegion(cfg.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, optionsFuncs...)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
		return nil, nil, err
	}
	s3OptionFuncs := make([]func(options *s3.Options), 0)
	if cfg.S3ForcePathStyle {
		s3OptionFuncs = append(s3OptionFuncs, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}
	if cfg.Endpoint != "" {
		s3OptionFuncs = append(s3OptionFuncs, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}
	client := s3.NewFromConfig(awsCfg, s3OptionFuncs...)

	return &s3ListObjectsAPIImpl{client: client}, client, nil
}

func (api *s3ListObjectsAPIImpl) NewListObjectsV2Paginator(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
	return s3.NewListObjectsV2Paginator(api.client, params)
}
