package ibmstorageprotectreceiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestStartAndShutdownReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.Config.StartAt = "beginning"
	sink := new(consumertest.MetricsSink)

	receiver, err := factory.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	err = receiver.Start(context.Background(), nil)
	require.NoError(t, err)

	md := testdata.GenerateMetrics(5)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	require.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0600)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllMetrics(), 1)
	assert.EqualValues(t, md, sink.AllMetrics()[0])

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}
