// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsefareceiver

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver/internal/metadata"
)

type mockSysFsReader struct {
	exists     bool
	existsErr  error
	devices    []string
	devicesErr error
	ports      map[string][]string
	portsErr   map[string]error
	counters   map[string]map[string]map[string]uint64 // device -> port -> counter -> value
	counterErr map[string]map[string]map[string]error  // device -> port -> counter -> error
	gids       map[string]string                       // device -> GID value
	gidErr     map[string]error                        // device -> GID error
}

func (m *mockSysFsReader) EfaDataExists() (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockSysFsReader) ListDevices() ([]string, error) {
	return m.devices, m.devicesErr
}

func (m *mockSysFsReader) ListPorts(deviceName string) ([]string, error) {
	if m.portsErr != nil {
		if err, ok := m.portsErr[deviceName]; ok {
			return nil, err
		}
	}
	return m.ports[deviceName], nil
}

func (m *mockSysFsReader) ReadGID(deviceName string) (string, error) {
	if m.gidErr != nil {
		if err, ok := m.gidErr[deviceName]; ok {
			return "", err
		}
	}
	if m.gids != nil {
		if gid, ok := m.gids[deviceName]; ok {
			return gid, nil
		}
	}
	return "", fmt.Errorf("no GID for device %s", deviceName)
}

func (m *mockSysFsReader) ReadCounter(deviceName, port, counter string) (uint64, error) {
	if m.counterErr != nil {
		if dev, ok := m.counterErr[deviceName]; ok {
			if p, ok := dev[port]; ok {
				if e, ok := p[counter]; ok {
					return 0, e
				}
			}
		}
	}
	if dev, ok := m.counters[deviceName]; ok {
		if p, ok := dev[port]; ok {
			if v, ok := p[counter]; ok {
				return v, nil
			}
		}
	}
	return 0, errCounterNotAvailable
}

type mockENIResolver struct {
	enis map[string]string // mac -> eni ID
}

func (m *mockENIResolver) GetENIID(macAddress string) (string, error) {
	if eni, ok := m.enis[macAddress]; ok {
		return eni, nil
	}
	return "", fmt.Errorf("no ENI for MAC %s", macAddress)
}

// zeroCounters returns a counter map with all known counters set to 0.
func zeroCounters() map[string]uint64 {
	m := make(map[string]uint64, len(efaCounters))
	for _, c := range efaCounters {
		m[c.name] = 0
	}
	return m
}

// withValues returns a copy of base with the given overrides applied.
func withValues(base, overrides map[string]uint64) map[string]uint64 {
	m := make(map[string]uint64, len(base))
	for k, v := range base {
		m[k] = v
	}
	for k, v := range overrides {
		m[k] = v
	}
	return m
}

func newTestMock() *mockSysFsReader {
	return &mockSysFsReader{
		exists:  true,
		devices: []string{"rdmap0s31", "rdmap1s31"},
		ports: map[string][]string{
			"rdmap0s31": {"1"},
			"rdmap1s31": {"1"},
		},
		counters: map[string]map[string]map[string]uint64{
			"rdmap0s31": {"1": withValues(zeroCounters(), map[string]uint64{
				"rdma_read_bytes":             1000,
				"rdma_write_bytes":            2000,
				"rdma_write_recv_bytes":       3000,
				"rx_bytes":                    4000,
				"rx_drops":                    5,
				"tx_bytes":                    5000,
				"retrans_bytes":               100,
				"retrans_pkts":                10,
				"retrans_timeout_events":      1,
				"unresponsive_remote_events":  2,
				"impaired_remote_conn_events": 3,
				"tx_pkts":                     400,
				"rx_pkts":                     500,
				"send_bytes":                  6000,
				"recv_bytes":                  7000,
				"send_wrs":                    800,
				"recv_wrs":                    900,
				"rdma_write_wrs":              110,
				"rdma_read_wrs":               120,
				"rdma_write_wr_err":           4,
				"rdma_read_wr_err":            6,
				"rdma_read_resp_bytes":        8000,
			})},
			"rdmap1s31": {"1": withValues(zeroCounters(), map[string]uint64{
				"rdma_read_bytes":       6000,
				"rdma_write_bytes":      7000,
				"rdma_write_recv_bytes": 8000,
				"rx_bytes":              9000,
				"tx_bytes":              10000,
				"tx_pkts":               200,
				"rx_pkts":               300,
				"send_bytes":            11000,
				"recv_bytes":            12000,
				"send_wrs":              150,
				"recv_wrs":              160,
				"rdma_write_wrs":        50,
				"rdma_read_wrs":         60,
				"rdma_write_wr_err":     1,
				"rdma_read_wr_err":      2,
				"rdma_read_resp_bytes":  13000,
			})},
		},
	}
}

func TestScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = newTestMock()

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	// With data point attributes, all metrics go into a single ResourceMetrics
	require.GreaterOrEqual(t, metrics.ResourceMetrics().Len(), 1)

	totalDataPoints := 0
	devicesSeen := make(map[string]bool)
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				for dp := 0; dp < m.Sum().DataPoints().Len(); dp++ {
					totalDataPoints++
					pt := m.Sum().DataPoints().At(dp)
					device, ok := pt.Attributes().Get("aws.efa.device")
					assert.True(t, ok, "expected aws.efa.device attribute on data point")
					devicesSeen[device.Str()] = true

					_, hasPort := pt.Attributes().Get("aws.efa.port")
					assert.True(t, hasPort, "expected aws.efa.port attribute on data point")
				}
			}
		}
	}

	assert.True(t, devicesSeen["rdmap0s31"], "expected data points for rdmap0s31")
	assert.True(t, devicesSeen["rdmap1s31"], "expected data points for rdmap1s31")
	// 22 counters per device * 2 devices = 44 data points total
	assert.Equal(t, 44, totalDataPoints)
}

func TestScrapeWithENIResolution(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{
		exists:  true,
		devices: []string{"rdmap0s31"},
		ports:   map[string][]string{"rdmap0s31": {"1"}},
		counters: map[string]map[string]map[string]uint64{
			"rdmap0s31": {"1": zeroCounters()},
		},
		gids: map[string]string{
			"rdmap0s31": "fe80::200:ff:fe00:1",
		},
	}
	s.eniResolver = &mockENIResolver{
		enis: map[string]string{"00:00:00:00:00:01": "eni-abc123"},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.GreaterOrEqual(t, metrics.ResourceMetrics().Len(), 1)

	// Check that eni.id appears as a data point attribute
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	m := sm.Metrics().At(0)
	pt := m.Sum().DataPoints().At(0)
	eniID, ok := pt.Attributes().Get("aws.efa.eni.id")
	assert.True(t, ok, "expected aws.efa.eni.id attribute on data point")
	assert.Equal(t, "eni-abc123", eniID.Str())
}

func TestScrapeENIResolutionRetryOnFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{
		exists:  true,
		devices: []string{"efa0"},
		ports:   map[string][]string{"efa0": {"1"}},
		counters: map[string]map[string]map[string]uint64{
			"efa0": {"1": zeroCounters()},
		},
		gids: map[string]string{"efa0": "fe80::200:ff:fe00:1"},
	}

	// First scrape: ENI resolver fails
	s.eniResolver = &mockENIResolver{enis: map[string]string{}}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.GreaterOrEqual(t, metrics.ResourceMetrics().Len(), 1)
	// eni.id should be empty string (still present as attribute but empty)
	rm := metrics.ResourceMetrics().At(0)
	pt := rm.ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0)
	eniID, ok := pt.Attributes().Get("aws.efa.eni.id")
	assert.True(t, ok, "expected aws.efa.eni.id attribute on data point")
	assert.Empty(t, eniID.Str(), "expected empty eni.id on first scrape (IMDS failure)")

	// Second scrape: ENI resolver succeeds — should retry since failure wasn't cached
	s.eniResolver = &mockENIResolver{
		enis: map[string]string{"00:00:00:00:00:01": "eni-retry123"},
	}

	metrics, err = s.scrape(t.Context())
	require.NoError(t, err)
	require.GreaterOrEqual(t, metrics.ResourceMetrics().Len(), 1)
	rm = metrics.ResourceMetrics().At(0)
	pt = rm.ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0)
	eniID, ok = pt.Attributes().Get("aws.efa.eni.id")
	assert.True(t, ok, "expected aws.efa.eni.id on second scrape (retry succeeded)")
	assert.Equal(t, "eni-retry123", eniID.Str())
}

func TestScrapeNoEfaDevices(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{exists: false}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)

	err := s.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, s.reader)
}

func TestScrapeMetricValues(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{
		exists:  true,
		devices: []string{"rdmap0s31"},
		ports:   map[string][]string{"rdmap0s31": {"1"}},
		counters: map[string]map[string]map[string]uint64{
			"rdmap0s31": {"1": withValues(zeroCounters(), map[string]uint64{
				"rdma_read_bytes":      42,
				"tx_pkts":              100,
				"rx_pkts":              200,
				"send_bytes":           300,
				"recv_bytes":           400,
				"send_wrs":             500,
				"recv_wrs":             600,
				"rdma_write_wrs":       700,
				"rdma_read_wrs":        800,
				"rdma_write_wr_err":    9,
				"rdma_read_wr_err":     10,
				"rdma_read_resp_bytes": 1100,
			})},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.GreaterOrEqual(t, metrics.ResourceMetrics().Len(), 1)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)

	expected := map[string]int64{
		"efa_rdma_read_bytes":      42,
		"efa_tx_pkts":              100,
		"efa_rx_pkts":              200,
		"efa_send_bytes":           300,
		"efa_recv_bytes":           400,
		"efa_send_wrs":             500,
		"efa_recv_wrs":             600,
		"efa_rdma_write_wrs":       700,
		"efa_rdma_read_wrs":        800,
		"efa_rdma_write_wr_err":    9,
		"efa_rdma_read_wr_err":     10,
		"efa_rdma_read_resp_bytes": 1100,
	}

	found := make(map[string]bool)
	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		if expectedVal, ok := expected[m.Name()]; ok {
			assert.Equal(t, expectedVal, m.Sum().DataPoints().At(0).IntValue(),
				"unexpected value for metric %s", m.Name())
			found[m.Name()] = true
		}
	}

	for name := range expected {
		assert.True(t, found[name], "expected to find metric %s", name)
	}
}

func TestScrapeListDevicesError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	s := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	s.reader = &mockSysFsReader{
		exists:     true,
		devicesErr: errors.New("permission denied"),
	}

	_, err := s.scrape(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read EFA devices")
}

func TestScrapeEfaDataExistsError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{exists: false, existsErr: errors.New("permission denied")}

	metrics, err := s.scrape(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check EFA data")
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestScrapePartialDeviceFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{
		exists:  true,
		devices: []string{"efa0", "efa1"},
		ports:   map[string][]string{"efa0": {"1"}},
		portsErr: map[string]error{
			"efa1": errors.New("device removed"),
		},
		counters: map[string]map[string]map[string]uint64{
			"efa0": {"1": withValues(zeroCounters(), map[string]uint64{"rdma_read_bytes": 100})},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	// Only efa0 should have data points
	totalDataPoints := 0
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			for k := 0; k < rm.ScopeMetrics().At(j).Metrics().Len(); k++ {
				totalDataPoints += rm.ScopeMetrics().At(j).Metrics().At(k).Sum().DataPoints().Len()
			}
		}
	}
	assert.Equal(t, 22, totalDataPoints, "expected 22 data points from efa0 only")
}

func TestScrapeCounterReadError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{
		exists:  true,
		devices: []string{"efa0", "efa1"},
		ports: map[string][]string{
			"efa0": {"1"},
			"efa1": {"1"},
		},
		counters: map[string]map[string]map[string]uint64{
			"efa0": {"1": zeroCounters()},
			"efa1": {"1": withValues(zeroCounters(), map[string]uint64{"rdma_read_bytes": 100})},
		},
		counterErr: map[string]map[string]map[string]error{
			"efa0": {"1": {"rx_bytes": errors.New("I/O error")}},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	// Both devices should have data points (efa0 with partial counters)
	totalDataPoints := 0
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			for k := 0; k < rm.ScopeMetrics().At(j).Metrics().Len(); k++ {
				totalDataPoints += rm.ScopeMetrics().At(j).Metrics().At(k).Sum().DataPoints().Len()
			}
		}
	}
	// efa0: 21 counters (rx_bytes failed) + efa1: 22 counters = 43
	assert.Equal(t, 43, totalDataPoints)
}

func TestRecordOverflow(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{
		exists:  true,
		devices: []string{"efa0"},
		ports:   map[string][]string{"efa0": {"1"}},
		counters: map[string]map[string]map[string]uint64{
			"efa0": {"1": withValues(zeroCounters(), map[string]uint64{
				"rdma_read_bytes": math.MaxUint64,
				"tx_bytes":        200,
			})},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.GreaterOrEqual(t, metrics.ResourceMetrics().Len(), 1)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)

	// rdma_read_bytes overflows int64, so it's skipped: 21 instead of 22
	totalDataPoints := 0
	for i := 0; i < sm.Metrics().Len(); i++ {
		totalDataPoints += sm.Metrics().At(i).Sum().DataPoints().Len()
	}
	assert.Equal(t, 21, totalDataPoints)

	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		if m.Name() == "efa_tx_bytes" {
			assert.Equal(t, int64(200), m.Sum().DataPoints().At(0).IntValue())
		}
		if m.Name() == "efa_rdma_read_bytes" {
			assert.Equal(t, 0, m.Sum().DataPoints().Len(), "efa_rdma_read_bytes should have no data points")
		}
	}
}

func TestScrapePartialCounterAvailability(t *testing.T) {
	// Simulate an older EFA driver that only has 15 of 22 counters.
	// Missing counters return errCounterNotAvailable, so readCounters
	// skips them. The scraper should emit exactly 15 data points per device.
	partialCounters := map[string]uint64{
		"rdma_read_bytes":             100,
		"rdma_write_bytes":            200,
		"rdma_write_recv_bytes":       300,
		"rx_bytes":                    400,
		"rx_drops":                    5,
		"tx_bytes":                    500,
		"retrans_bytes":               50,
		"retrans_pkts":                10,
		"retrans_timeout_events":      1,
		"unresponsive_remote_events":  2,
		"impaired_remote_conn_events": 3,
		"tx_pkts":                     60,
		"rx_pkts":                     70,
		"send_bytes":                  600,
		"recv_bytes":                  700,
	}

	missingCounters := []string{
		"send_wrs", "recv_wrs", "rdma_write_wrs", "rdma_read_wrs",
		"rdma_write_wr_err", "rdma_read_wr_err", "rdma_read_resp_bytes",
	}
	missingErrs := make(map[string]error, len(missingCounters))
	for _, c := range missingCounters {
		missingErrs[c] = errCounterNotAvailable
	}

	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.reader = &mockSysFsReader{
		exists:  true,
		devices: []string{"efa0", "efa1"},
		ports: map[string][]string{
			"efa0": {"1"},
			"efa1": {"1"},
		},
		counters: map[string]map[string]map[string]uint64{
			"efa0": {"1": partialCounters},
			"efa1": {"1": partialCounters},
		},
		counterErr: map[string]map[string]map[string]error{
			"efa0": {"1": missingErrs},
			"efa1": {"1": missingErrs},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	// Count data points per device by checking aws.efa.device attribute
	deviceDataPoints := make(map[string]int)
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				for dp := 0; dp < m.Sum().DataPoints().Len(); dp++ {
					pt := m.Sum().DataPoints().At(dp)
					device, _ := pt.Attributes().Get("aws.efa.device")
					deviceDataPoints[device.Str()]++
				}
			}
		}
	}

	assert.Equal(t, 15, deviceDataPoints["efa0"], "expected 15 data points for efa0 with partial counters")
	assert.Equal(t, 15, deviceDataPoints["efa1"], "expected 15 data points for efa1 with partial counters")
}
