// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"
const testObjectsDir = "./testdata/e2e/testobjects/"
const expectedDir = "./testdata/e2e/expected/"

type objAction int

const (
	create          objAction = iota
	createAndDelete           // create and delete first, then poll for logs
	none
)

func TestE2E(t *testing.T) {

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.HTTP = nil
	logsConsumer := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, logsConsumer)
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	// startup collector in k8s cluster
	collectorObjs := k8stest.CreateCollectorObjects(t, dynamicClient, testID)

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(dynamicClient, obj), "failed to delete object %s", obj.GetName())
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
					newObj, err := k8stest.CreateObject(dynamicClient, obj)
					require.NoErrorf(t, err, "failed to create k8s object from file %s", fileName)
					testObjs = append(testObjs, newObj)
				}
				if tc.objectAction == createAndDelete {
					for _, obj := range testObjs {
						require.NoErrorf(t, k8stest.DeleteObject(dynamicClient, obj), "failed to delete object %s", obj.GetName())
					}
				} else {
					defer func() {
						for _, obj := range testObjs {
							require.NoErrorf(t, k8stest.DeleteObject(dynamicClient, obj), "failed to delete object %s", obj.GetName())
						}
					}()
				}
			}

			require.Eventuallyf(t, func() bool {
				return len(logsConsumer.AllLogs()) > 0
			}, time.Duration(tc.timeoutMinutes)*time.Minute, 1*time.Second,
				"Timeout: failed to receive logs in %d minutes", tc.timeoutMinutes)

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
