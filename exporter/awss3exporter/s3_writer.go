// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
)

func newUploadManager(
	ctx context.Context,
	conf *Config,
	metadata string,
	format string,
) (upload.Manager, error) {
	configOpts := []func(*config.LoadOptions) error{}

	if region := conf.S3Uploader.Region; region != "" {
		configOpts = append(configOpts, config.WithRegion(region))
	}
	if retryMode := conf.S3Uploader.RetryMode; retryMode != "" {
		configOpts = append(configOpts, config.WithRetryMode(aws.RetryMode(retryMode)))
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, err
	}

	s3Opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.EndpointOptions = s3.EndpointResolverOptions{
				DisableHTTPS: conf.S3Uploader.DisableSSL,
			}
			o.UsePathStyle = conf.S3Uploader.S3ForcePathStyle
		},
	}

	if conf.S3Uploader.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String((conf.S3Uploader.Endpoint))
		})
	}

	if arn := conf.S3Uploader.RoleArn; arn != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.Credentials = stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), arn)
		})
	}

	if endpoint := conf.S3Uploader.Endpoint; endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	if retryMaxAttempts := conf.S3Uploader.RetryMaxAttempts; retryMaxAttempts != 0 {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.Retryer = retry.AddWithMaxAttempts(o.Retryer, retryMaxAttempts)
		})
	}

	if retryMaxBackoff := conf.S3Uploader.RetryMaxBackoff; retryMaxBackoff != 0 {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.Retryer = retry.AddWithMaxBackoffDelay(o.Retryer, retryMaxBackoff)
		})
	}

	var managerOpts []upload.ManagerOpt
	if conf.S3Uploader.ACL != "" {
		managerOpts = append(managerOpts,
			upload.WithACL(s3types.ObjectCannedACL(conf.S3Uploader.ACL)))
	}

	return upload.NewS3Manager(
		conf.S3Uploader.S3Bucket,
		&upload.PartitionKeyBuilder{
			PartitionPrefix: conf.S3Uploader.S3Prefix,
			PartitionFormat: conf.S3Uploader.S3PartitionFormat,
			FilePrefix:      conf.S3Uploader.FilePrefix,
			Metadata:        metadata,
			FileFormat:      format,
			Compression:     conf.S3Uploader.Compression,
		},
		s3.NewFromConfig(cfg, s3Opts...),
		s3types.StorageClass(conf.S3Uploader.StorageClass),
		managerOpts...,
	), nil
}
