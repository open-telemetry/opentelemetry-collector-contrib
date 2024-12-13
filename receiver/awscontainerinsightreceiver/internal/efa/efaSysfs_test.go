// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package efa

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

type mockSysfsReader struct {
	scrapeCounts map[string]uint64
}

var _ sysFsReader = (*mockSysfsReader)(nil)

func newMockSysfsReader() mockSysfsReader {
	return mockSysfsReader{
		scrapeCounts: make(map[string]uint64),
	}
}

func (r mockSysfsReader) EfaDataExists() (bool, error) {
	return true, nil
}

func (r mockSysfsReader) ListDevices() ([]efaDeviceName, error) {
	return []efaDeviceName{"efa0", "efa1"}, nil
}

func (r mockSysfsReader) ListPorts(_ efaDeviceName) ([]string, error) {
	return []string{"1", "2"}, nil
}

var mockCounterValues = map[string]uint64{
	counterRdmaReadBytes:      1,
	counterRdmaWriteBytes:     2,
	counterRdmaWriteRecvBytes: 3,
	counterRxBytes:            4,
	counterRxDrops:            5,
	counterTxBytes:            6,
}

func (r mockSysfsReader) ReadCounter(deviceName efaDeviceName, port string, counter string) (uint64, error) {
	key := fmt.Sprintf("%s/%s/%s", deviceName, port, counter)
	value := mockCounterValues[counter] * (r.scrapeCounts[key] + 1)
	r.scrapeCounts[key]++
	return value, nil
}

type mockDecorator struct{}

var _ stores.Decorator = (*mockDecorator)(nil)

func (d mockDecorator) Decorate(metric stores.CIMetric) stores.CIMetric {
	metric.AddTag("decorated", "true")
	return metric
}

func (d mockDecorator) Shutdown() error {
	return nil
}

type mockPodResourcesStore struct{}

var _ podResourcesStore = (*mockPodResourcesStore)(nil)

func (p mockPodResourcesStore) AddResourceName(_ string) {}

func (p mockPodResourcesStore) GetContainerInfo(deviceID string, _ string) *stores.ContainerInfo {
	switch deviceID {
	case "efa0":
		return &stores.ContainerInfo{
			PodName:       "pod0",
			ContainerName: "container0",
			Namespace:     "namespace0",
		}
	case "efa1":
		return &stores.ContainerInfo{
			PodName:       "pod1",
			ContainerName: "container1",
			Namespace:     "namespace1",
		}
	}
	return nil
}

type expectation struct {
	fields map[string]uint64
	tags   map[string]string
}

var efa0Metrics = []expectation{
	{
		map[string]uint64{
			"node_efa_rdma_read_bytes":       2,
			"node_efa_rdma_write_bytes":      4,
			"node_efa_rdma_write_recv_bytes": 6,
			"node_efa_rx_bytes":              8,
			"node_efa_rx_dropped":            10,
			"node_efa_tx_bytes":              12,
		},
		map[string]string{
			ci.MetricType: ci.TypeNodeEFA,
			ci.EfaDevice:  "efa0",
			ci.Timestamp:  "to-be-replaced",
			"decorated":   "true",
		},
	},
	{
		map[string]uint64{
			"pod_efa_rdma_read_bytes":       2,
			"pod_efa_rdma_write_bytes":      4,
			"pod_efa_rdma_write_recv_bytes": 6,
			"pod_efa_rx_bytes":              8,
			"pod_efa_rx_dropped":            10,
			"pod_efa_tx_bytes":              12,
		},
		map[string]string{
			ci.MetricType:       ci.TypePodEFA,
			ci.EfaDevice:        "efa0",
			ci.K8sNamespace:     "namespace0",
			ci.PodNameKey:       "pod0",
			ci.ContainerNamekey: "container0",
			ci.Timestamp:        "to-be-replaced",
			"decorated":         "true",
		},
	},
	{
		map[string]uint64{
			"container_efa_rdma_read_bytes":       2,
			"container_efa_rdma_write_bytes":      4,
			"container_efa_rdma_write_recv_bytes": 6,
			"container_efa_rx_bytes":              8,
			"container_efa_rx_dropped":            10,
			"container_efa_tx_bytes":              12,
		},
		map[string]string{
			ci.MetricType:       ci.TypeContainerEFA,
			ci.EfaDevice:        "efa0",
			ci.K8sNamespace:     "namespace0",
			ci.PodNameKey:       "pod0",
			ci.ContainerNamekey: "container0",
			ci.Timestamp:        "to-be-replaced",
			"decorated":         "true",
		},
	},
}

var efa1NodeMetric = expectation{
	map[string]uint64{
		"node_efa_rdma_read_bytes":       2,
		"node_efa_rdma_write_bytes":      4,
		"node_efa_rdma_write_recv_bytes": 6,
		"node_efa_rx_bytes":              8,
		"node_efa_rx_dropped":            10,
		"node_efa_tx_bytes":              12,
	},
	map[string]string{
		ci.MetricType: ci.TypeNodeEFA,
		ci.EfaDevice:  "efa1",
		ci.Timestamp:  "to-be-replaced",
		"decorated":   "true",
	},
}

var efa1PodContainerMetrics = []expectation{
	{
		map[string]uint64{
			"pod_efa_rdma_read_bytes":       2,
			"pod_efa_rdma_write_bytes":      4,
			"pod_efa_rdma_write_recv_bytes": 6,
			"pod_efa_rx_bytes":              8,
			"pod_efa_rx_dropped":            10,
			"pod_efa_tx_bytes":              12,
		},
		map[string]string{
			ci.MetricType:       ci.TypePodEFA,
			ci.EfaDevice:        "efa1",
			ci.K8sNamespace:     "namespace1",
			ci.PodNameKey:       "pod1",
			ci.ContainerNamekey: "container1",
			ci.Timestamp:        "to-be-replaced",
			"decorated":         "true",
		},
	},
	{
		map[string]uint64{
			"container_efa_rdma_read_bytes":       2,
			"container_efa_rdma_write_bytes":      4,
			"container_efa_rdma_write_recv_bytes": 6,
			"container_efa_rx_bytes":              8,
			"container_efa_rx_dropped":            10,
			"container_efa_tx_bytes":              12,
		},
		map[string]string{
			ci.MetricType:       ci.TypeContainerEFA,
			ci.EfaDevice:        "efa1",
			ci.K8sNamespace:     "namespace1",
			ci.PodNameKey:       "pod1",
			ci.ContainerNamekey: "container1",
			ci.Timestamp:        "to-be-replaced",
			"decorated":         "true",
		},
	},
}

var efa1Metrics = []expectation{efa1NodeMetric, efa1PodContainerMetrics[0], efa1PodContainerMetrics[1]}

func TestGetMetrics(t *testing.T) {
	s := NewEfaSyfsScraper(zap.NewNop(), mockDecorator{}, mockPodResourcesStore{})
	s.sysFsReader = newMockSysfsReader()

	var expectedMetrics []expectation
	expectedMetrics = append(expectedMetrics, efa0Metrics...)
	expectedMetrics = append(expectedMetrics, efa1Metrics...)

	// first metric collection should return no metrics because we haven't established a baseline for the delta
	// calculation yet
	assert.NoError(t, s.scrape())
	result := s.GetMetrics()
	assert.Empty(t, result)

	assert.NoError(t, s.scrape())
	result = s.GetMetrics()
	checkExpectations(t, expectedMetrics, result)
}

func TestGetMetricsBeforeSuccessfulScrape(t *testing.T) {
	s := NewEfaSyfsScraper(zap.NewNop(), mockDecorator{}, mockPodResourcesStore{})

	result := s.GetMetrics()
	assert.Empty(t, result)
	result = s.GetMetrics()
	assert.Empty(t, result)
	checkExpectations(t, []expectation{}, result)
}

type mockPodResourcesStoreMissingOneDevice struct{}

var _ podResourcesStore = (*mockPodResourcesStoreMissingOneDevice)(nil)

func (p mockPodResourcesStoreMissingOneDevice) AddResourceName(_ string) {}

func (p mockPodResourcesStoreMissingOneDevice) GetContainerInfo(deviceID string, _ string) *stores.ContainerInfo {
	if deviceID == "efa0" {
		return &stores.ContainerInfo{
			PodName:       "pod0",
			ContainerName: "container0",
			Namespace:     "namespace0",
		}
	}
	return nil
}

func TestGetMetricsMissingDeviceFromPodResources(t *testing.T) {
	s := NewEfaSyfsScraper(zap.NewNop(), mockDecorator{}, mockPodResourcesStoreMissingOneDevice{})
	s.sysFsReader = newMockSysfsReader()

	assert.NoError(t, s.scrape())
	assert.Empty(t, s.GetMetrics())

	var expectedMetrics []expectation
	expectedMetrics = append(expectedMetrics, efa0Metrics...)
	expectedMetrics = append(expectedMetrics, efa1NodeMetric)

	assert.NoError(t, s.scrape())
	result := s.GetMetrics()
	checkExpectations(t, expectedMetrics, result)
}

func checkExpectations(t *testing.T, expected []expectation, actual []pmetric.Metrics) {
	assert.Equal(t, len(expected), len(actual))
	if len(expected) == 0 {
		return
	}

	slices.SortFunc(expected, func(a, b expectation) int {
		return compareStrings(a.tags[ci.EfaDevice], b.tags[ci.EfaDevice])
	})
	slices.SortFunc(actual, func(a, b pmetric.Metrics) int {
		aVal, _ := a.ResourceMetrics().At(0).Resource().Attributes().Get(ci.EfaDevice)
		bVal, _ := b.ResourceMetrics().At(0).Resource().Attributes().Get(ci.EfaDevice)
		return compareStrings(aVal.Str(), bVal.Str())
	})

	for i := 0; i < len(expected); i++ {
		expectedMetric := expected[i]
		actualMetric := actual[i]
		checkExpectation(t, expectedMetric.fields, expectedMetric.tags, actualMetric)
	}
}

func compareStrings(x, y string) int {
	if x < y {
		return -1
	}
	if x > y {
		return +1
	}
	return 0
}

func checkExpectation(t *testing.T, expectedFields map[string]uint64, expectedTags map[string]string, md pmetric.Metrics) {
	rm := md.ResourceMetrics()
	assert.Equal(t, 1, rm.Len())
	resource := rm.At(0)

	sm := resource.ScopeMetrics()
	assert.Equal(t, len(expectedFields), sm.Len())
	for i := 0; i < sm.Len(); i++ {
		metrics := sm.At(i).Metrics()
		assert.Equal(t, 1, metrics.Len())
		metric := metrics.At(0)
		assert.Contains(t, expectedFields, metric.Name())
		dps := metric.Gauge().DataPoints()
		assert.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		assert.Equal(t, int64(expectedFields[metric.Name()]), dp.IntValue())
	}

	attrs := resource.Resource().Attributes()
	assert.Equal(t, len(expectedTags), attrs.Len())
	if _, ok := expectedTags[ci.Timestamp]; ok {
		timestampString, timestamp := findTimestamp(t, attrs)
		assert.NotNil(t, timestamp)
		age := time.Since(timestamp)
		assert.Less(t, age, time.Minute)
		expectedTags[ci.Timestamp] = timestampString
	}
	for key, value := range expectedTags {
		actual, ok := attrs.Get(key)
		assert.True(t, ok)
		assert.Equal(t, value, actual.Str())
	}
}

func findTimestamp(t *testing.T, attrs pcommon.Map) (string, time.Time) {
	var timestampString string
	var timestamp time.Time
	attrs.Range(func(k string, v pcommon.Value) bool {
		if k == ci.Timestamp {
			ms, err := strconv.ParseInt(v.Str(), 10, 64)
			assert.NoError(t, err)
			timestampString = v.Str()
			timestamp = time.Unix(ms/1000, 0)
			return false
		}
		return true
	})
	return timestampString, timestamp
}

func TestScrape(t *testing.T) {
	s := NewEfaSyfsScraper(zap.NewNop(), nil, mockPodResourcesStore{})
	s.sysFsReader = newMockSysfsReader()

	expectedCounters := efaCounters{
		// All values multiplied by 2 because we mock 2 ports
		rdmaReadBytes:      2,
		rdmaWriteBytes:     4,
		rdmaWriteRecvBytes: 6,
		rxBytes:            8,
		rxDrops:            10,
		txBytes:            12,
	}
	expected := efaDevices{
		"efa0": &expectedCounters,
		"efa1": &expectedCounters,
	}

	assert.NoError(t, s.scrape())
	assert.Equal(t, expected, *s.store.devices)
}

type mockSysfsReaderError1 struct{}

func (r mockSysfsReaderError1) EfaDataExists() (bool, error) {
	return false, errors.New("mocked error")
}

func (r mockSysfsReaderError1) ListDevices() ([]efaDeviceName, error) {
	return []efaDeviceName{"efa0"}, nil
}

func (r mockSysfsReaderError1) ListPorts(_ efaDeviceName) ([]string, error) {
	return []string{}, nil
}

func (r mockSysfsReaderError1) ReadCounter(_ efaDeviceName, _ string, _ string) (uint64, error) {
	return 0, nil
}

type mockSysfsReaderError2 struct{}

func (r mockSysfsReaderError2) EfaDataExists() (bool, error) {
	return true, nil
}

func (r mockSysfsReaderError2) ListDevices() ([]efaDeviceName, error) {
	return nil, errors.New("mocked error")
}

func (r mockSysfsReaderError2) ListPorts(_ efaDeviceName) ([]string, error) {
	return []string{}, nil
}

func (r mockSysfsReaderError2) ReadCounter(_ efaDeviceName, _ string, _ string) (uint64, error) {
	return 0, nil
}

type mockSysfsReaderError3 struct{}

func (r mockSysfsReaderError3) EfaDataExists() (bool, error) {
	return true, nil
}

func (r mockSysfsReaderError3) ListDevices() ([]efaDeviceName, error) {
	return []efaDeviceName{"efa0"}, nil
}

func (r mockSysfsReaderError3) ListPorts(_ efaDeviceName) ([]string, error) {
	return nil, errors.New("mocked error")
}

func (r mockSysfsReaderError3) ReadCounter(_ efaDeviceName, _ string, _ string) (uint64, error) {
	return 0, nil
}

type mockSysfsReaderError4 struct{}

func (r mockSysfsReaderError4) EfaDataExists() (bool, error) {
	return true, nil
}

func (r mockSysfsReaderError4) ListDevices() ([]efaDeviceName, error) {
	return []efaDeviceName{"efa0"}, nil
}

func (r mockSysfsReaderError4) ListPorts(_ efaDeviceName) ([]string, error) {
	return []string{"1"}, nil
}

func (r mockSysfsReaderError4) ReadCounter(_ efaDeviceName, _ string, _ string) (uint64, error) {
	return 1, errors.New("mocked error")
}

func TestScrapeErrors(t *testing.T) {
	for _, reader := range []sysFsReader{mockSysfsReaderError1{}, mockSysfsReaderError2{}, mockSysfsReaderError3{}, mockSysfsReaderError4{}} {
		s := NewEfaSyfsScraper(zap.NewNop(), nil, mockPodResourcesStore{})
		s.sysFsReader = reader

		assert.Error(t, s.scrape())
		assert.Nil(t, s.store.devices)
	}
}

type mockSysfsReaderNoEfaData struct{}

func (r mockSysfsReaderNoEfaData) EfaDataExists() (bool, error) {
	return false, nil
}

func (r mockSysfsReaderNoEfaData) ListDevices() ([]efaDeviceName, error) {
	return nil, errors.New("mocked error")
}

func (r mockSysfsReaderNoEfaData) ListPorts(_ efaDeviceName) ([]string, error) {
	return nil, errors.New("mocked error")
}

func (r mockSysfsReaderNoEfaData) ReadCounter(_ efaDeviceName, _ string, _ string) (uint64, error) {
	return 0, errors.New("mocked error")
}

func TestScrapeNoEfaData(t *testing.T) {
	s := NewEfaSyfsScraper(zap.NewNop(), nil, mockPodResourcesStore{})
	s.sysFsReader = mockSysfsReaderNoEfaData{}

	assert.NoError(t, s.scrape())
	assert.Nil(t, s.store.devices)
}
