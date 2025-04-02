// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

// NewCluster returns a fake Kafka cluster and configkafka.ClientConfig
// with the default configuration, and brokers set to the cluster addresses.
func NewCluster(tb testing.TB, opts ...kfake.Opt) (*kfake.Cluster, configkafka.ClientConfig) {
	cluster, err := kfake.NewCluster(opts...)
	require.NoError(tb, err)
	tb.Cleanup(cluster.Close)

	cfg := configkafka.NewDefaultClientConfig()
	cfg.Brokers = cluster.ListenAddrs()
	return cluster, cfg
}
