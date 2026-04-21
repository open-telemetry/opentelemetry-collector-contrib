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
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	ingestedTag    = "otel-collector:status"
	ingestedStatus = "ingested"
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
	GetObjectTagging(ctx context.Context, params *s3.GetObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error)
	PutObjectTagging(ctx context.Context, params *s3.PutObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error)
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

// tagS3Object tags an S3 object for a given bucket and key
func tagS3Object(ctx context.Context, client SingleObjectAPI, bucket, key string) error {
	params := s3.PutObjectTaggingInput{
		Bucket: &bucket,
		Key:    &key,
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{
					Key:   aws.String(ingestedTag),
					Value: aws.String(ingestedStatus),
				},
			},
		},
	}
	_, err := client.PutObjectTagging(ctx, &params)
	return err
}

// hasIngestedTag checks if an S3 object has the ingested tag set
func hasIngestedTag(ctx context.Context, client SingleObjectAPI, bucket, key string) (bool, error) {
	params := s3.GetObjectTaggingInput{
		Bucket: &bucket,
		Key:    &key,
	}
	output, err := client.GetObjectTagging(ctx, &params)
	if err != nil {
		return false, err
	}

	for _, tag := range output.TagSet {
		if tag.Key != nil && *tag.Key == ingestedTag &&
			tag.Value != nil && *tag.Value == ingestedStatus {
			return true, nil
		}
	}
	return false, nil
}
