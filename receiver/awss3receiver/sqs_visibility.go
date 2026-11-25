// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.uber.org/zap"
)

var errVisibilityExpired = errors.New("SQS message visibility expired")

func (r *s3SQSNotificationReader) startVisibilityRenewal(ctx context.Context, message types.Message, receivedAt time.Time) (context.Context, func() error) {
	if r.visibilityTimeoutSeconds == 0 {
		return ctx, func() error { return ctx.Err() }
	}
	processingCtx, cancelProcessing := context.WithCancelCause(ctx)
	renewalCtx, cancelRenewal := context.WithCancel(ctx)
	done := make(chan struct{})
	duration := time.Duration(r.visibilityTimeoutSeconds) * time.Second
	expiresAt := time.Now().Add(duration)
	go func() {
		defer close(done)
		timer := time.NewTimer(duration / 2)
		defer timer.Stop()
		for {
			select {
			case <-renewalCtx.Done():
				return
			case <-timer.C:
				if !time.Now().Before(expiresAt) {
					cancelProcessing(errVisibilityExpired)
					return
				}
				// SQS allows at most 12 hours from ReceiveMessage. Reserve one
				// second for transit and rounding when approaching that limit.
				remaining := int32((12*time.Hour - time.Since(receivedAt) - time.Second) / time.Second)
				timeout := min(r.visibilityTimeoutSeconds, remaining)
				if timeout <= 0 {
					cancelProcessing(errVisibilityExpired)
					return
				}
				requestCtx, cancelRequest := context.WithTimeout(renewalCtx, min(time.Until(expiresAt), 30*time.Second))
				started := time.Now()
				_, err := r.sqsClient.ChangeMessageVisibility(requestCtx, &sqs.ChangeMessageVisibilityInput{
					QueueUrl: aws.String(r.queueURL), ReceiptHandle: message.ReceiptHandle,
					VisibilityTimeout: timeout,
				})
				cancelRequest()
				if err != nil {
					if renewalCtx.Err() == nil {
						r.logger.Warn("Failed to renew SQS message visibility", zap.Error(err))
						cancelProcessing(err)
					}
					return
				}
				expiresAt = started.Add(time.Duration(timeout) * time.Second)
				timer.Reset(time.Until(expiresAt) / 2)
			}
		}
	}()
	var once sync.Once
	var renewalErr error
	return processingCtx, func() error {
		once.Do(func() {
			cancelRenewal()
			<-done
			renewalErr = context.Cause(processingCtx)
			if renewalErr == nil && !time.Now().Before(expiresAt) {
				renewalErr = errVisibilityExpired
			}
			cancelProcessing(nil)
		})
		return renewalErr
	}
}
