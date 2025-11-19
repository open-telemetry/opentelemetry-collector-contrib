// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

// s3API abstracts out the S3 APIs allowing mocking for tests
type s3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Service define services exposed for consumers
type S3Service interface {
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
