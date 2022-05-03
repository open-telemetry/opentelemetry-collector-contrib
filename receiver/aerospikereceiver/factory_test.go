package aerospikereceiver_test

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	factory := aerospikereceiver.NewFactory()
	require.Equal(t, "aerospike", string(factory.Type()))
	cfg := factory.CreateDefaultConfig().(*aerospikereceiver.Config)
	require.Equal(t, time.Minute, cfg.CollectionInterval)
	require.False(t, cfg.CollectClusterMetrics)
}
