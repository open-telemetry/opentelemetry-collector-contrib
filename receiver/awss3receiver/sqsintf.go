// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSClient defines the SQS operations used by s3SQSNotificationReader
type SQSClient interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	DeleteQueue(ctx context.Context, params *sqs.DeleteQueueInput, optFns ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error)
}

// newSQSClient creates a new SQS client with the provided configuration
func newSQSClient(ctx context.Context, region string, endpoint string) (SQSClient, error) {
	optionsFuncs := make([]func(*config.LoadOptions) error, 0)
	if region != "" {
		optionsFuncs = append(optionsFuncs, config.WithRegion(region))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, optionsFuncs...)
	if err != nil {
		return nil, err
	}

	sqsOptionFuncs := make([]func(options *sqs.Options), 0)
	if endpoint != "" {
		sqsOptionFuncs = append(sqsOptionFuncs, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	return sqs.NewFromConfig(awsCfg, sqsOptionFuncs...), nil
}
