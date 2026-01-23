// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package processscraper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

// testdataProcesses is the list of valid lead processes within the
// testdata/procfs folder. New processes must be added here to be usable
// within tests.
var testdataProcesses = map[string]int32{
	"context switches": 828531,
}

// testdataProcessHandles will create a list of processHandles from the list of valid
// lead processes within testdata/procfs. This needs to be constructed per-process
// from a global list to account for some particular features of the actual procfs in
// the kernel (namely responding // differently to getdents(2) to only return lead threads)
// that we can't mock in a way that our scraper and gopsutil will recognize. Instead, the
// list is constructed by each process in the valid lead process list and wrapped in our own
// interfaces the same way that the scraper would.
func testdataProcessHandles(ctx context.Context) (processHandles, error) {
	processes := make([]wrappedProcessHandle, 0, len(testdataProcesses))
	for _, pid := range testdataProcesses {
		p, err := process.NewProcessWithContext(ctx, pid)
		// We ignore ErrorProcessNotRunning; since this is mock data there
		// won't actually be a process running for each pid (technically
		// possible that one's local testing environment actually is using
		// the pid but we don't care whether that's true or not).
		if err != nil && !errors.Is(err, process.ErrorProcessNotRunning) {
			return nil, err
		}
		processes = append(processes, wrappedProcessHandle{Process: p})
	}

	return &gopsProcessHandles{handles: processes}, nil
}

// getTestdataPid can be used to fetch a particular processHandle by pid from
// a list of processHandles. Most tests will use this to get a particular handle
// from the constructed list of processHandles from testdata/procfs.
func getTestdataPid(handles processHandles, pid int32) processHandle {
	for i := range handles.Len() {
		if handles.Pid(i) == pid {
			return handles.At(i)
		}
	}
	return nil
}

func newTestProcessScraper(ctx context.Context, t *testing.T) *processScraper {
	t.Helper()
	metricsCfg := metadata.DefaultMetricsBuilderConfig()
	metricsCfg.Metrics = metricsConfigAllEnabled()
	cfg := &Config{
		MetricsBuilderConfig: metricsCfg,
	}
	scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, scraper.start(ctx, componenttest.NewNopHost()))
	return scraper
}

// TestProcessScraper_Procfs will create a processscraper that reads the data in testdata/procfs.
// This processscraper can be passed to various tests to test individual metrics on particular
// processes from the procfs directory.
//
// The testdata/procfs directory is given to gopsutil as if it's a real procfs directory. This
// means when gopsutil scrapes information from a particular process in the directory, it will
// actually parse the particular files of concern and parse them from the directory.
//
// The challenge with testing on a real procfs is that setting up the data to have deterministic
// results, especially for values that will constantly change throughout a real running process
// lifetime, ranges from non-trivial to impossible. These tests allow us to process real procfs
// data, but since it's tracked in source the results can be asserted against deterministically.
func TestProcessScraper_Procfs(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	require.NoError(t, err)
	ctx := context.WithValue(
		t.Context(),
		common.EnvKey,
		common.EnvMap{common.HostProcEnvKey: filepath.Join(wd, "testdata", "procfs")},
	)

	scraper := newTestProcessScraper(ctx, t)

	handles, err := testdataProcessHandles(ctx)
	require.NoError(t, err)

	runTestdataProcfsTest(ctx, t, scraper, handles, "context switches", testContextSwitches)
}

type testdataProcfsTestFunc func(context.Context, *testing.T, *processScraper, processHandles)

func runTestdataProcfsTest(
	ctx context.Context,
	t *testing.T,
	scraper *processScraper,
	handles processHandles,
	name string,
	testFunc testdataProcfsTestFunc,
) {
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		testFunc(ctx, t, scraper, handles)
	})
}

// testContextSwitches tests process 828531. The process has 3 tasks (including itself)
// and they should all be counted in the resulting metric:
//
// 828531:
//
//	voluntary_ctxt_switches:	0
//	nonvoluntary_ctxt_switches:	2415
//
// 828532:
//
//	voluntary_ctxt_switches:	190
//	nonvoluntary_ctxt_switches:	3
//
// 828533:
//
//	voluntary_ctxt_switches:	190
//	nonvoluntary_ctxt_switches:	5
func testContextSwitches(ctx context.Context, t *testing.T, scraper *processScraper, handles processHandles) {
	ctxSwitchProcess := getTestdataPid(handles, testdataProcesses["context switches"])
	require.NoError(t, scraper.scrapeAndAppendContextSwitchMetrics(ctx, pcommon.NewTimestampFromTime(time.Now()), ctxSwitchProcess))

	metrics := scraper.mb.Emit()
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	res := metrics.ResourceMetrics().At(0)
	require.Equal(t, 1, res.ScopeMetrics().Len())
	scope := res.ScopeMetrics().At(0)
	ms := scope.Metrics()
	var foundVoluntarySwitches, foundInvoluntarySwitches bool
	for i := range ms.Len() {
		m := ms.At(i)
		require.Equal(t, m.Name(), metadata.MetricsInfo.ProcessContextSwitches.Name)
		dps := m.Sum().DataPoints()
		for k := range dps.Len() {
			dp := dps.At(k)
			attrs := dp.Attributes()
			// NOTE: Would love this attribute key name to be encoded in metadata somewhere but it's not.
			ctxSwitchType, ok := attrs.Get("type")
			require.True(t, ok)
			switch ctxSwitchType.AsString() {
			case metadata.AttributeContextSwitchTypeInvoluntary.String():
				foundInvoluntarySwitches = true
				// Total expected Involuntary Context Switches: 2415 + 3 + 5 = 2423
				require.Equal(t, int64(2423), dp.IntValue())

			case metadata.AttributeContextSwitchTypeVoluntary.String():
				foundVoluntarySwitches = true
				// Total expected Voluntary Context Switches: 0 + 190 + 190 = 380
				require.Equal(t, int64(380), dp.IntValue())
			}
		}
	}
	require.True(t, foundInvoluntarySwitches && foundVoluntarySwitches)
}

// metricsConfigAllEnabled uses reflection to mark all metrics in
// metadata.MetricsConfig as Enabled. This should be resilient to
// new metrics being added.
func metricsConfigAllEnabled() metadata.MetricsConfig {
	cfg := metadata.DefaultMetricsConfig()
	v := reflect.ValueOf(&cfg).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Type() == reflect.TypeFor[metadata.MetricConfig]() {
			field.FieldByName("Enabled").SetBool(true)
		}
	}
	return cfg
}
