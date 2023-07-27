// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package perfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
)

// MockPerfCounterScraperError is an implementation of PerfCounterScraper that returns
// the supplied errors when scrape, GetObject, or GetValues are called.
type MockPerfCounterScraperError struct {
	scrapeErr    error
	getObjectErr error
	getValuesErr error
	initError    error
}

// NewMockPerfCounterScraperError returns a MockPerfCounterScraperError that will return
// the specified errors on subsequent function calls.
func NewMockPerfCounterScraperError(scrapeErr, getObjectErr, getValuesErr, initError error) *MockPerfCounterScraperError {
	return &MockPerfCounterScraperError{scrapeErr: scrapeErr, getObjectErr: getObjectErr, getValuesErr: getValuesErr, initError: initError}
}

// start is a no-op
func (p *MockPerfCounterScraperError) Initialize(_ ...string) error {
	if p.initError != nil {
		return p.initError
	}
	return nil
}

// scrape returns the specified scrapeErr or an object that will return a subsequent error
// if scrapeErr is nil
func (p *MockPerfCounterScraperError) Scrape() (PerfDataCollection, error) {
	if p.scrapeErr != nil {
		return nil, p.scrapeErr
	}

	return mockPerfDataCollectionError{getObjectErr: p.getObjectErr, getValuesErr: p.getValuesErr}, nil
}

type mockPerfDataCollectionError struct {
	getObjectErr error
	getValuesErr error
}

// GetObject returns the specified getObjectErr or an object that will return a subsequent
// error if getObjectErr is nil
func (p mockPerfDataCollectionError) GetObject(_ string) (PerfDataObject, error) {
	if p.getObjectErr != nil {
		return nil, p.getObjectErr
	}

	return mockPerfDataObjectError{getValuesErr: p.getValuesErr}, nil
}

type mockPerfDataObjectError struct {
	getValuesErr error
}

// Filter is a no-op
func (obj mockPerfDataObjectError) Filter(_, _ filterset.FilterSet, _ bool) {
}

// GetValues returns the specified getValuesErr
func (obj mockPerfDataObjectError) GetValues(_ ...string) ([]*CounterValues, error) {
	return nil, obj.getValuesErr
}

// MockPerfCounterScraper is an implementation of PerfCounterScraper that returns the supplied
// object / counter values on each successive call to scrape, in the specified order.
//
// Example Usage:
//
//	s := NewMockPerfCounterScraper(map[string]map[string][]int64{
//	    "Object1": map[string][]int64{
//	        "Counter1": []int64{1, 2},
//	        "Counter2": []int64{4},
//	    },
//	})
//
// s.scrape().GetObject("Object1").GetValues("Counter1", "Counter2")
//
// ... 1st call returns []*CounterValues{ { Values: { "Counter1": 1, "Counter2": 4 } } }
// ... 2nd call returns []*CounterValues{ { Values: { "Counter1": 2, "Counter2": 4 } } }
type MockPerfCounterScraper struct {
	objectsAndValuesToReturn map[string]map[string][]int64
	timesCalled              int
}

// NewMockPerfCounterScraper returns a MockPerfCounterScraper that will return the supplied
// object / counter values on each successive call to scrape, in the specified order.
func NewMockPerfCounterScraper(objectsAndValuesToReturn map[string]map[string][]int64) *MockPerfCounterScraper {
	return &MockPerfCounterScraper{objectsAndValuesToReturn: objectsAndValuesToReturn}
}

// start is a no-op
func (p *MockPerfCounterScraper) Initialize(_ ...string) error {
	return nil
}

// scrape returns a perf data collection with the supplied object / counter values,
// according to the supplied order.
func (p *MockPerfCounterScraper) Scrape() (PerfDataCollection, error) {
	objectsAndValuesToReturn := make(map[string]map[string]int64, len(p.objectsAndValuesToReturn))
	for objectName, countersToReturn := range p.objectsAndValuesToReturn {
		valuesToReturn := make(map[string]int64, len(countersToReturn))
		for counterName, orderedValuesToReturn := range countersToReturn {
			returnIndex := p.timesCalled
			if returnIndex >= len(orderedValuesToReturn) {
				returnIndex = len(orderedValuesToReturn) - 1
			}
			valuesToReturn[counterName] = orderedValuesToReturn[returnIndex]
		}
		objectsAndValuesToReturn[objectName] = valuesToReturn
	}

	p.timesCalled++
	return mockPerfDataCollection{objectsAndValuesToReturn: objectsAndValuesToReturn}, nil
}

type mockPerfDataCollection struct {
	objectsAndValuesToReturn map[string]map[string]int64
}

// GetObject returns the specified object / counter values
func (p mockPerfDataCollection) GetObject(objectName string) (PerfDataObject, error) {
	valuesToReturn, ok := p.objectsAndValuesToReturn[objectName]
	if !ok {
		return nil, fmt.Errorf("Unable to find object %q", objectName)
	}

	return mockPerfDataObject{valuesToReturn: valuesToReturn}, nil
}

type mockPerfDataObject struct {
	valuesToReturn map[string]int64
}

// Filter is a no-op
func (obj mockPerfDataObject) Filter(_, _ filterset.FilterSet, _ bool) {
}

// GetValues returns the specified counter values
func (obj mockPerfDataObject) GetValues(counterNames ...string) ([]*CounterValues, error) {
	value := &CounterValues{Values: map[string]int64{}}
	for _, counterName := range counterNames {
		valueToReturn, ok := obj.valuesToReturn[counterName]
		if !ok {
			return nil, fmt.Errorf("Mock Perf Counter Scraper configured incorrectly. Return value for counter %q not specified", counterName)
		}
		value.Values[counterName] = valueToReturn
	}
	return []*CounterValues{value}, nil
}
