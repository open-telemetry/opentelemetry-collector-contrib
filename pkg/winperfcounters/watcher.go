// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package winperfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"

import (
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters/internal/third_party/telegraf/win_perf_counters"
)

const totalInstanceName = "_Total"

var _ PerfCounterWatcher = (*perfCounter)(nil)

// PerfCounterWatcher represents how to scrape data
type PerfCounterWatcher interface {
	// Path returns the counter path
	Path() string
	// ScrapeData collects a measurement and returns the value(s).
	ScrapeData() ([]CounterValue, error)
	// ScrapeRawValue collects a measurement and returns the raw value.
	ScrapeRawValue(rawValue *int64) (bool, error)
	// ScrapeRawValues collects a measurement and returns the raw value(s) for all instances.
	ScrapeRawValues() ([]RawCounterValue, error)
	// Resets the perfcounter query.
	Reset() error
	// Close all counters/handles related to the query and free all associated memory.
	Close() error
}

type (
	CounterValue    = win_perf_counters.CounterValue
	RawCounterValue = win_perf_counters.RawCounterValue
)

type perfCounter struct {
	path   string
	query  win_perf_counters.PerformanceQuery
	handle win_perf_counters.PDH_HCOUNTER
}

// NewWatcher creates new PerfCounterWatcher by provided parts of its path.
func NewWatcher(object, instance, counterName string) (PerfCounterWatcher, error) {
	return NewWatcherFromPath(counterPath(object, instance, counterName))
}

// NewWatcherFromPath creates new PerfCounterWatcher by provided path.
func NewWatcherFromPath(path string) (PerfCounterWatcher, error) {
	counter, err := newPerfCounter(path, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create perf counter with path %v: %w", path, err)
	}
	return counter, nil
}

func counterPath(object, instance, counterName string) string {
	if instance != "" {
		instance = fmt.Sprintf("(%s)", instance)
	}

	return fmt.Sprintf("\\%s%s\\%s", object, instance, counterName)
}

// newPerfCounter returns a new performance counter for the specified descriptor.
func newPerfCounter(counterPath string, collectOnStartup bool) (*perfCounter, error) {
	query, handle, err := initQuery(counterPath, collectOnStartup)
	if err != nil {
		return nil, err
	}

	counter := &perfCounter{
		path:   counterPath,
		query:  query,
		handle: *handle,
	}

	return counter, nil
}

func initQuery(counterPath string, collectOnStartup bool) (*win_perf_counters.PerformanceQueryImpl, *win_perf_counters.PDH_HCOUNTER, error) {
	query := &win_perf_counters.PerformanceQueryImpl{}
	err := query.Open()
	if err != nil {
		return nil, nil, err
	}

	var handle win_perf_counters.PDH_HCOUNTER
	handle, err = query.AddEnglishCounterToQuery(counterPath)
	if err != nil {
		return nil, nil, err
	}

	// Some perf counters (e.g. cpu) return the usage stats since the last measure.
	// We collect data on startup to avoid an invalid initial reading
	if collectOnStartup {
		err = query.CollectData()
		if err != nil {
			// Ignore PDH_NO_DATA error, it is expected when there are no
			// matching instances.
			var pdhErr *win_perf_counters.PdhError
			if !errors.As(err, &pdhErr) || pdhErr.ErrorCode != win_perf_counters.PDH_NO_DATA {
				return nil, nil, err
			}
		}
	}

	return query, &handle, nil
}

// Reset re-creates the PerformanceCounter query and if the operation succeeds, closes the previous query.
// This is useful when scraping wildcard counters.
func (pc *perfCounter) Reset() error {
	query, handle, err := initQuery(pc.path, true)
	if err != nil {
		return err
	}
	_ = pc.Close()
	pc.query = query
	pc.handle = *handle
	return nil
}

func (pc *perfCounter) Close() error {
	return pc.query.Close()
}

func (pc *perfCounter) Path() string {
	return pc.path
}

func (pc *perfCounter) ScrapeData() ([]CounterValue, error) {
	hasData, err := pc.collectDataForScrape()
	if err != nil {
		return nil, err
	}
	if !hasData {
		return nil, nil
	}

	vals, err := pc.query.GetFormattedCounterArrayDouble(pc.handle)
	if err != nil {
		return nil, fmt.Errorf("failed to format data for performance counter '%s': %w", pc.path, err)
	}

	vals = cleanupScrapedValues(vals)
	return vals, nil
}

func (pc *perfCounter) ScrapeRawValues() ([]RawCounterValue, error) {
	hasData, err := pc.collectDataForScrape()
	if err != nil {
		return nil, err
	}
	if !hasData {
		return nil, nil
	}

	vals, err := pc.query.GetRawCounterArray(pc.handle)
	if err != nil {
		return nil, fmt.Errorf("failed to get raw data for performance counter '%s': %w", pc.path, err)
	}

	vals = cleanupScrapedValues(vals)
	return vals, nil
}

func (pc *perfCounter) ScrapeRawValue(rawValue *int64) (bool, error) {
	*rawValue = 0
	hasData, err := pc.collectDataForScrape()
	if err != nil {
		return false, err
	}
	if !hasData {
		return false, nil
	}

	*rawValue, err = pc.query.GetRawCounterValue(pc.handle)
	if err != nil {
		return false, fmt.Errorf("failed to get raw data for performance counter '%s': %w", pc.path, err)
	}

	return true, nil
}

// ExpandWildCardPath examines the local computer and returns those counter paths that match the given counter path which contains wildcard characters.
func ExpandWildCardPath(counterPath string) ([]string, error) {
	return win_perf_counters.ExpandWildCardPath(counterPath)
}

func getInstanceName(ctr any) string {
	switch v := ctr.(type) {
	case win_perf_counters.CounterValue:
		return v.InstanceName
	case win_perf_counters.RawCounterValue:
		return v.InstanceName
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

func setInstanceName(ctr any, name string) {
	switch v := ctr.(type) {
	case *win_perf_counters.CounterValue:
		v.InstanceName = name
	case *win_perf_counters.RawCounterValue:
		v.InstanceName = name
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// cleanupScrapedValues handles instance name collisions and standardizes names.
// It cleans up the list in-place to avoid unnecessary copies.
func cleanupScrapedValues[C CounterValue | RawCounterValue](vals []C) []C {
	if len(vals) == 0 {
		return vals
	}

	// If there is only one "_Total" instance, clear the instance name.
	if len(vals) == 1 && getInstanceName(vals[0]) == totalInstanceName {
		setInstanceName(&vals[0], "")
		return vals
	}

	occurrences := map[string]int{}
	totalIndex := -1

	for i := range vals {
		instanceName := getInstanceName(vals[i])

		if instanceName == totalInstanceName {
			// Remember if a "_Total" instance was present.
			totalIndex = i
		}

		if n, ok := occurrences[instanceName]; ok {
			// Append indices to duplicate instance names.
			occurrences[instanceName]++
			setInstanceName(&vals[i], fmt.Sprintf("%s#%d", instanceName, n))
		} else {
			occurrences[instanceName] = 1
		}
	}

	// Remove the "_Total" instance, as it can be computed with a sum aggregation.
	if totalIndex >= 0 {
		return removeItemAt(vals, totalIndex)
	}

	return vals
}

func removeItemAt[C CounterValue | RawCounterValue](vals []C, idx int) []C {
	vals[idx] = vals[len(vals)-1]
	var zeroValue C
	vals[len(vals)-1] = zeroValue
	return vals[:len(vals)-1]
}

func (pc *perfCounter) collectDataForScrape() (bool, error) {
	if err := pc.query.CollectData(); err != nil {
		var pdhErr *win_perf_counters.PdhError
		if !errors.As(err, &pdhErr) || (pdhErr.ErrorCode != win_perf_counters.PDH_NO_DATA && pdhErr.ErrorCode != win_perf_counters.PDH_CALC_NEGATIVE_DENOMINATOR) {
			return false, fmt.Errorf("failed to collect data for performance counter '%s': %w", pc.path, err)
		}

		if pdhErr.ErrorCode == win_perf_counters.PDH_NO_DATA {
			// No data is available for the counter, so no error but also no data
			return false, nil
		}

		if pdhErr.ErrorCode == win_perf_counters.PDH_CALC_NEGATIVE_DENOMINATOR {
			// A counter rolled over, so the value is invalid
			// See https://support.microfocus.com/kb/doc.php?id=7010545
			// Wait one second and retry once
			time.Sleep(time.Second)
			if retryErr := pc.query.CollectData(); retryErr != nil {
				return false, fmt.Errorf("failed retry for performance counter '%s': %w", pc.path, err)
			}
		}
	}

	return true, nil
}
