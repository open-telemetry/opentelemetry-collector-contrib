// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"

import (
	"context"

	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"github.com/googleapis/gax-go/v2"
)

// subscriberClient subset of `pubsub.SubscriberClient`
type SubscriberClient interface {
	Close() error
	StreamingPull(ctx context.Context, opts ...gax.CallOption) (pubsubpb.Subscriber_StreamingPullClient, error)
}
