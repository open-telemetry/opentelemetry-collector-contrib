// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		Trace: ReceiverOption{
			Topic:        defaultTopicName,
			Encoding:     defaultEncoding,
			ConsumerName: defaultConsumerName,
			Subscription: defaultSubscription,
		},
		Metric: ReceiverOption{
			Topic:        defaultTopicName,
			Encoding:     defaultEncoding,
			ConsumerName: defaultConsumerName,
			Subscription: defaultSubscription,
		},
		Log: ReceiverOption{
			Topic:        defaultTopicName,
			Encoding:     defaultEncoding,
			ConsumerName: defaultConsumerName,
			Subscription: defaultSubscription,
		},
		Endpoint:       defaultServiceURL,
		Authentication: Authentication{},
	}, cfg)
}

func TestValidateReceiverOption(t *testing.T) {
	opt := ReceiverOption{Topic: "pulsar"}
	err := opt.validate()
	require.NoError(t, err)
	require.Equal(t, opt.Topic, "pulsar")
	require.Equal(t, opt.Encoding, defaultEncoding)
	require.Equal(t, opt.Subscription, defaultSubscription)
	require.Equal(t, opt.ConsumerName, defaultConsumerName)
}
