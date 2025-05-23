// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/ecsmock"
)

// Simply start and stop, the actual test logic is in sd_test.go until we implement the ListWatcher interface.
// In that case sd itself does not use timer and relies on caller to trigger List.
func TestExtensionStartStop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping flaky test on Windows, see " +
			"https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4042")
	}
	refreshInterval := 100 * time.Millisecond

	createTestExt := func(c *ecsmock.Cluster, output string) extension.Extension {
		f := newTestTaskFetcher(t, c, c)
		cfg := createDefaultConfig()
		sdCfg := cfg.(*Config)
		sdCfg.RefreshInterval = refreshInterval
		sdCfg.ResultFile = output
		cs := extensiontest.NewNopSettings(extensiontest.NopType)
		ext, err := createExtensionWithFetcher(cs, sdCfg, f)
		require.NoError(t, err)
		return ext
	}

	t.Run("noop", func(t *testing.T) {
		c := ecsmock.NewCluster()
		ext := createTestExt(c, "testdata/ut_ext_noop.actual.yaml")
		require.IsType(t, &ecsObserver{}, ext)
		require.NoError(t, ext.Start(context.TODO(), &nopHost{
			reportFunc: func(event *componentstatus.Event) {
				require.NoError(t, event.Err())
			},
		}))
		require.NoError(t, ext.Shutdown(context.TODO()))
	})

	t.Run("critical error", func(t *testing.T) {
		c := ecsmock.NewClusterWithName("different than default config")
		f := newTestTaskFetcher(t, c, c)
		cfg := createDefaultConfig()
		sdCfg := cfg.(*Config)
		sdCfg.RefreshInterval = 100 * time.Millisecond
		sdCfg.ResultFile = "testdata/ut_ext_critical_error.actual.yaml"
		cs := extensiontest.NewNopSettings(extensiontest.NopType)
		statusEventChan := make(chan *componentstatus.Event)
		ext, err := createExtensionWithFetcher(cs, sdCfg, f)
		require.NoError(t, err)
		err = ext.Start(context.Background(), &nopHost{
			reportFunc: func(e *componentstatus.Event) {
				statusEventChan <- e
			},
		})
		require.NoError(t, err)
		e := <-statusEventChan
		require.Error(t, e.Err())
		require.Error(t, hasCriticalError(zap.NewExample(), e.Err()))
	})
}

var _ componentstatus.Reporter = (*nopHost)(nil)

type nopHost struct {
	reportFunc func(event *componentstatus.Event)
}

func (nh *nopHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *nopHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
