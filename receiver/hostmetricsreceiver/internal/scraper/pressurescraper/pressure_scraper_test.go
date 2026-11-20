// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pressurescraper

import (
	"context"
	"errors"
	"io/fs"
	"syscall"
	"testing"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

func TestScrapeErrors(t *testing.T) {
	stats := procfs.PSIStats{
		Some: &procfs.PSILine{Total: 1000000},
		Full: &procfs.PSILine{Total: 500000},
	}

	tests := []struct {
		name           string
		errs           map[string]error
		expectedLen    int
		expectedErr    string
		expectedFailed int
	}{
		{
			name:        "irq file missing",
			errs:        map[string]error{"irq": &fs.PathError{Op: "open", Path: "/proc/pressure/irq", Err: fs.ErrNotExist}},
			expectedLen: 5,
		},
		{
			name:        "irq time accounting disabled",
			errs:        map[string]error{"irq": &fs.PathError{Op: "read", Path: "/proc/pressure/irq", Err: syscall.EOPNOTSUPP}},
			expectedLen: 5,
		},
		{
			name:           "irq read error",
			errs:           map[string]error{"irq": &fs.PathError{Op: "read", Path: "/proc/pressure/irq", Err: syscall.EACCES}},
			expectedLen:    5,
			expectedErr:    "permission denied",
			expectedFailed: 1,
		},
		{
			name: "psi disabled",
			errs: map[string]error{
				"cpu":    syscall.EOPNOTSUPP,
				"memory": syscall.EOPNOTSUPP,
				"io":     syscall.EOPNOTSUPP,
				"irq":    syscall.EOPNOTSUPP,
			},
			expectedErr:    "operation not supported",
			expectedFailed: 5,
		},
		{
			name:           "memory error",
			errs:           map[string]error{"memory": errors.New("err")},
			expectedLen:    4,
			expectedErr:    "err",
			expectedFailed: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newPressureScraper(scrapertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
			s.psiStats = func(_ context.Context, resource string) (procfs.PSIStats, error) {
				if err := test.errs[resource]; err != nil {
					return procfs.PSIStats{}, err
				}
				return stats, nil
			}
			require.NoError(t, s.start(t.Context(), componenttest.NewNopHost()))

			md, err := s.scrape(t.Context())
			assert.Len(t, sumValues(md), test.expectedLen)

			if test.expectedFailed == 0 {
				require.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, test.expectedErr)
			var partialErr scrapererror.PartialScrapeError
			require.ErrorAs(t, err, &partialErr)
			assert.Equal(t, test.expectedFailed, partialErr.Failed)
		})
	}
}

func TestStartTime(t *testing.T) {
	s := newPressureScraper(scrapertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
	s.bootTime = func(context.Context) (uint64, error) { return 100, nil }
	s.psiStats = func(context.Context, string) (procfs.PSIStats, error) {
		return procfs.PSIStats{Some: &procfs.PSILine{Total: 1}, Full: &procfs.PSILine{Total: 1}}, nil
	}
	require.NoError(t, s.start(t.Context(), componenttest.NewNopHost()))

	md, err := s.scrape(t.Context())
	require.NoError(t, err)

	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	require.Equal(t, 6, metrics.Len())
	for i := 0; i < metrics.Len(); i++ {
		assert.Equal(t, pcommon.Timestamp(100*1e9), metrics.At(i).Sum().DataPoints().At(0).StartTimestamp(), metrics.At(i).Name())
	}
}

func TestBootTimeError(t *testing.T) {
	s := newPressureScraper(scrapertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
	s.bootTime = func(context.Context) (uint64, error) { return 0, errors.New("err1") }
	assert.EqualError(t, s.start(t.Context(), componenttest.NewNopHost()), "err1")
}

func sumValues(md pmetric.Metrics) map[string]float64 {
	values := map[string]float64{}
	if md.ResourceMetrics().Len() == 0 {
		return values
	}
	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		values[m.Name()] = m.Sum().DataPoints().At(0).DoubleValue()
	}
	return values
}
