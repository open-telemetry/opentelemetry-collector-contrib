// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/ecsmock"
)

// inspectErrorHost implements component.Host.
// btw: I only find assertNoErrorHost in other components, seems there is no exported util struct.
type inspectErrorHost struct {
	component.Host

	// Why we need a mutex here? Our extension only has one go routine so it seems
	// we don't need to protect the error as our extension is the only component for this 'host'.
	// But without the lock the test actually fails on race detector.
	// There is no actual concurrency in our test, when we read the error in test assertion,
	// we know the extension has already stopped because we provided invalided config and waited long enough.
	// However, (I assume) from race detector's perspective, race between a stopped goroutine and a running one
	// is same as two running goroutines. A goroutine's stop condition is uncertain at runtime, and the data
	// access order may varies, goroutine A can stop before B in first run and reverse in next run.
	// As long as there is some read/write of one memory area without protection from multiple go routines,
	// it means the code can have data race, but it does not mean this race always happen.
	// In our case, the race never happens because we hard coded the sleep time of two go routines.
	//
	// btw: assertNoErrorHost does not have mutex because it never saves the error. Its ReportFatalError
	// just call assertion and forget about nil error. For unexpected error it call helpers to fail the test
	// and those helper func all have mutex. https://golang.org/src/testing/testing.go
	mu  sync.Mutex
	err error
}

func newInspectErrorHost() component.Host {
	return &inspectErrorHost{
		Host: componenttest.NewNopHost(),
	}
}

func (h *inspectErrorHost) ReportFatalError(err error) {
	h.mu.Lock()
	h.err = err
	h.mu.Unlock()
}

func (h *inspectErrorHost) getError() error {
	h.mu.Lock()
	cp := h.err
	h.mu.Unlock()
	return cp
}

// Simply start and stop, the actual test logic is in sd_test.go until we implement the ListWatcher interface.
// In that case sd itself does not use timer and relies on caller to trigger List.
func TestExtensionStartStop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping flaky test on Windows, see " +
			"https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4042")
	}
	refreshInterval := 100 * time.Millisecond
	waitDuration := 2 * refreshInterval

	createTestExt := func(c *ecsmock.Cluster, output string) extension.Extension {
		f := newTestTaskFetcher(t, c)
		cfg := createDefaultConfig()
		sdCfg := cfg.(*Config)
		sdCfg.RefreshInterval = refreshInterval
		sdCfg.ResultFile = output
		ext, err := createExtensionWithFetcher(extensiontest.NewNopCreateSettings(), sdCfg, f)
		require.NoError(t, err)
		return ext
	}

	t.Run("noop", func(t *testing.T) {
		c := ecsmock.NewCluster()
		ext := createTestExt(c, "testdata/ut_ext_noop.actual.yaml")
		require.IsType(t, &ecsObserver{}, ext)
		host := newInspectErrorHost()
		require.NoError(t, ext.Start(context.TODO(), host))
		time.Sleep(waitDuration)
		require.NoError(t, host.(*inspectErrorHost).getError())
		require.NoError(t, ext.Shutdown(context.TODO()))
	})

	t.Run("critical error", func(t *testing.T) {
		c := ecsmock.NewClusterWithName("different than default config")
		f := newTestTaskFetcher(t, c)
		cfg := createDefaultConfig()
		sdCfg := cfg.(*Config)
		sdCfg.RefreshInterval = 100 * time.Millisecond
		sdCfg.ResultFile = "testdata/ut_ext_critical_error.actual.yaml"
		cs := extensiontest.NewNopCreateSettings()
		statusEventChan := make(chan *component.StatusEvent)
		cs.TelemetrySettings.ReportComponentStatus = func(e *component.StatusEvent) error {
			statusEventChan <- e
			return nil
		}
		ext, err := createExtensionWithFetcher(cs, sdCfg, f)
		require.NoError(t, err)
		err = ext.Start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		e := <-statusEventChan
		require.Error(t, e.Err())
		require.Error(t, hasCriticalError(zap.NewExample(), e.Err()))
	})
}
