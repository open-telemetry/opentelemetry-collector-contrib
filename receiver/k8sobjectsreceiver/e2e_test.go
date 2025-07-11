// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sobjectsreceiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"
	testObjectsDir = "./testdata/e2e/testobjects/"
	expectedDir    = "./testdata/e2e/expected/"
)

type objAction int

const (
	create          objAction = iota
	createAndDelete           // create and delete first, then poll for logs
	none
)

// getOrInsertDefault is a helper function to get or insert a default value for a configoptional.Optional type.
func getOrInsertDefault[T any](t *testing.T, opt *configoptional.Optional[T]) *T {
	if opt.HasValue() {
		return opt.Get()
	}

	empty := confmap.NewFromStringMap(map[string]any{})
	require.NoError(t, empty.Unmarshal(opt))
	val := opt.Get()
	require.NotNil(t, "Expected a default value to be set for %T", val)
	return val
}

func TestE2E(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	logsConsumer := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, logsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	// startup collector in k8s cluster
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, "", map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	tests := []struct {
		name             string
		objectFileNames  []string
		objectAction     objAction
		expectedFileName string
		timeoutMinutes   int // timeout to wait for data
	}{
		{
			name:             "pull object",
			objectFileNames:  []string{},
			objectAction:     none,
			expectedFileName: "pull_namespaces.yaml",
			timeoutMinutes:   2,
		},
		{
			name:             "watch events",
			objectFileNames:  []string{"event.yaml"},
			objectAction:     create,
			expectedFileName: "watch_events.yaml",
			timeoutMinutes:   1,
		},
		{
			name:             "watch events in core/v1 version",
			objectFileNames:  []string{"event_core.yaml"},
			objectAction:     createAndDelete,
			expectedFileName: "watch_events_core.yaml",
			timeoutMinutes:   1,
		},
		{
			name:             "watch object",
			objectFileNames:  []string{"namespace.yaml"},
			objectAction:     create,
			expectedFileName: "watch_namespaces.yaml",
			timeoutMinutes:   2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				time.Sleep(10 * time.Second)
				logsConsumer.Reset() // reset consumer at the end since pull results are available at start
			}()

			expectedFile := filepath.Join(expectedDir, tc.expectedFileName)
			var expected plog.Logs
			expected, err = golden.ReadLogs(expectedFile)
			require.NoErrorf(t, err, "Error reading logs from %s", expectedFile)

			// additional k8s objs
			if tc.objectAction != none {
				testObjs := make([]*unstructured.Unstructured, 0, len(tc.objectFileNames))
				for _, fileName := range tc.objectFileNames {
					obj, err := os.ReadFile(filepath.Join(testObjectsDir, fileName))
					require.NoErrorf(t, err, "failed to read object file %s", fileName)
					newObj, err := k8stest.CreateObject(k8sClient, obj)
					require.NoErrorf(t, err, "failed to create k8s object from file %s", fileName)
					testObjs = append(testObjs, newObj)
				}
				if tc.objectAction == createAndDelete {
					for _, obj := range testObjs {
						require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
					}
				} else {
					defer func() {
						for _, obj := range testObjs {
							require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
						}
					}()
				}
			}

			require.Eventuallyf(t, func() bool {
				return len(logsConsumer.AllLogs()) > 0
			}, time.Duration(tc.timeoutMinutes)*time.Minute, 1*time.Second,
				"Timeout: failed to receive logs in %d minutes", tc.timeoutMinutes)

			// golden.WriteLogs(t, expectedFile, logsConsumer.AllLogs()[0])

			require.NoErrorf(t, plogtest.CompareLogs(expected, logsConsumer.AllLogs()[0],
				plogtest.IgnoreObservedTimestamp(),
				plogtest.IgnoreResourceLogsOrder(),
				plogtest.IgnoreScopeLogsOrder(),
				plogtest.IgnoreLogRecordsOrder()),
				"Received logs did not match log records in file %s", expectedFile,
			)
		})
	}
}
