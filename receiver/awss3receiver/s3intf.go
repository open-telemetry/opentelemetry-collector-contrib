// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"io"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ListObjectsV2Pager interface {
	HasMorePages() bool
	NextPage(context.Context, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type ListObjectsAPI interface {
	NewListObjectsV2Paginator(params *s3.ListObjectsV2Input) ListObjectsV2Pager
}

type SingleObjectAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type s3ListObjectsAPIImpl struct {
	client *s3.Client
}

func newS3Client(ctx context.Context, cfg S3DownloaderConfig) (ListObjectsAPI, SingleObjectAPI, error) {
	optionsFuncs := make([]func(*config.LoadOptions) error, 0)
	if cfg.Region != "" {
		optionsFuncs = append(optionsFuncs, config.WithRegion(cfg.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, optionsFuncs...)
	if err != nil {
		log.Printf("unable to load SDK config: %v", err)
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

// retrieveS3Object retrieves S3 object content for a given bucket and key
func retrieveS3Object(ctx context.Context, client SingleObjectAPI, bucket, key string) ([]byte, error) {
	params := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	output, err := client.GetObject(ctx, &params)
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	contents, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

// deleteS3Object deletes an S3 object for a given bucket and key
func deleteS3Object(ctx context.Context, client SingleObjectAPI, bucket, key string) error {
	params := s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := client.DeleteObject(ctx, &params)
	return err
}
