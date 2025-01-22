// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"context"
	"fmt"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"
)

// wrappedSubscriberClient allows to override the close function
type wrappedSubscriberClient struct {
	internal.SubscriberClient
	closeFn func() error
}

func (c *wrappedSubscriberClient) Close() error {
	if c.closeFn != nil {
		return c.closeFn()
	}
	return c.SubscriberClient.Close()
}

func newSubscriberClient(ctx context.Context, config *Config, userAgent string) (internal.SubscriberClient, error) {
	clientOptions, closeFn, err := generateClientOptions(config, userAgent)
	if err != nil {
		return nil, fmt.Errorf("failed preparing the gRPC client options to PubSub: %w", err)
	}

	client, err := pubsub.NewSubscriberClient(ctx, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed creating the gRPC client to PubSub: %w", err)
	}

	if closeFn == nil {
		return client, nil
	}

	return &wrappedSubscriberClient{
		SubscriberClient: client,
		closeFn:          closeFn,
	}, nil
}

func generateClientOptions(config *Config, userAgent string) ([]option.ClientOption, func() error, error) {
	var copts []option.ClientOption
	var closeFn func() error

	if userAgent != "" {
		copts = append(copts, option.WithUserAgent(userAgent))
	}
	if config.Endpoint != "" {
		if config.Insecure {
			var dialOpts []grpc.DialOption
			if userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(userAgent))
			}
			client, err := grpc.NewClient(config.Endpoint, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
			if err != nil {
				return nil, nil, err
			}
			copts = append(copts, option.WithGRPCConn(client))
			closeFn = client.Close // we need to be able to properly close the grpc client otherwise it'll leak goroutines
		} else {
			copts = append(copts, option.WithEndpoint(config.Endpoint))
		}
	}
	return copts, closeFn, nil
}
