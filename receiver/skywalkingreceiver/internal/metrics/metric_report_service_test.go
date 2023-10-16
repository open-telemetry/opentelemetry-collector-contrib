package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	transport = "fakeTransport"
)

func TestMetricGenerateBySendingMetric(t *testing.T) {
	c := &agent.JVMMetricCollection{
		ServiceInstance: "instance",
		Service:         "service",
		Metrics: []*agent.JVMMetric{
			{
				Time: time.Now().Unix(),
				Cpu: &common.CPU{
					UsagePercent: 0.4,
				},
				Memory: []*agent.Memory{
					{
						IsHeap:    true,
						Init:      1,
						Max:       1,
						Used:      1,
						Committed: 1,
					},
				},
				MemoryPool: []*agent.MemoryPool{
					{
						Type:      agent.PoolType_CODE_CACHE_USAGE,
						Init:      1,
						Max:       1,
						Used:      1,
						Committed: 1,
					},
				},
				Gc: []*agent.GC{
					{
						Phase: agent.GCPhase_NORMAL,
						Count: 1,
						Time:  time.Now().Unix(),
					},
				},
				Thread: &agent.Thread{
					LiveCount:                    1,
					DaemonCount:                  1,
					PeakCount:                    1,
					RunnableStateThreadCount:     1,
					BlockedStateThreadCount:      1,
					WaitingStateThreadCount:      1,
					TimedWaitingStateThreadCount: 1,
				},
				Clazz: &agent.Class{
					LoadedClassCount:        1,
					TotalUnloadedClassCount: 1,
					TotalLoadedClassCount:   1,
				},
			},
		},
	}

	_ = featuregate.GlobalRegistry().Set("telemetry.useOtelForInternalMetrics", true)
	fakeReceiver := component.NewID("fakeReceiver")
	tt, err := obsreporttest.SetupTelemetry(fakeReceiver)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.Background())) })

	rec, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             fakeReceiver,
		Transport:              transport,
		ReceiverCreateSettings: receiver.CreateSettings{ID: fakeReceiver, TelemetrySettings: tt.TelemetrySettings, BuildInfo: component.NewDefaultBuildInfo()},
	})
	err = consumeMetrics(context.Background(), c, consumertest.NewNop(), rec)
	require.NoError(t, err)
	require.NoError(t, tt.CheckReceiverMetrics(transport, 8, 0))
}
