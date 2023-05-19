// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aerospikereceiver_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"
)

func TestNewFactory(t *testing.T) {
	factory := aerospikereceiver.NewFactory()
	require.Equal(t, "aerospike", string(factory.Type()))
	cfg := factory.CreateDefaultConfig().(*aerospikereceiver.Config)
	require.Equal(t, time.Minute, cfg.CollectionInterval)
	require.False(t, cfg.CollectClusterMetrics)
}
