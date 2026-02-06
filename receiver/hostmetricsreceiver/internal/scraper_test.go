// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestGenerateServiceInstanceID(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		// Same hostname should produce the same UUID
		id1 := generateServiceInstanceID("myhost.example.com")
		id2 := generateServiceInstanceID("myhost.example.com")
		assert.Equal(t, id1, id2, "same hostname should produce same UUID")
	})

	t.Run("unique for different hostnames", func(t *testing.T) {
		id1 := generateServiceInstanceID("host1.example.com")
		id2 := generateServiceInstanceID("host2.example.com")
		assert.NotEqual(t, id1, id2, "different hostnames should produce different UUIDs")
	})

	t.Run("valid UUID format", func(t *testing.T) {
		id := generateServiceInstanceID("myhost.example.com")
		// UUID v5 format: xxxxxxxx-xxxx-5xxx-xxxx-xxxxxxxxxxxx
		assert.Len(t, id, 36, "UUID should be 36 characters")
		assert.Equal(t, '-', rune(id[8]), "UUID should have hyphen at position 8")
		assert.Equal(t, '-', rune(id[13]), "UUID should have hyphen at position 13")
		assert.Equal(t, '5', rune(id[14]), "UUID v5 should have '5' at position 14")
	})
}

func TestGenerateProcessServiceInstanceID(t *testing.T) {
	baseID := generateServiceInstanceID("myhost.example.com")

	t.Run("deterministic", func(t *testing.T) {
		// Same inputs should produce the same UUID
		id1 := generateProcessServiceInstanceID(baseID, 1234)
		id2 := generateProcessServiceInstanceID(baseID, 1234)
		assert.Equal(t, id1, id2, "same inputs should produce same UUID")
	})

	t.Run("unique for different PIDs", func(t *testing.T) {
		id1 := generateProcessServiceInstanceID(baseID, 1234)
		id2 := generateProcessServiceInstanceID(baseID, 5678)
		assert.NotEqual(t, id1, id2, "different PIDs should produce different UUIDs")
	})

	t.Run("different from base ID", func(t *testing.T) {
		processID := generateProcessServiceInstanceID(baseID, 1234)
		assert.NotEqual(t, baseID, processID, "process ID should differ from base ID")
	})
}

// mockScraper is a test scraper that returns predefined metrics
type mockScraper struct {
	metrics pmetric.Metrics
	started bool
	stopped bool
}

func (m *mockScraper) Start(_ context.Context, _ component.Host) error {
	m.started = true
	return nil
}

func (m *mockScraper) ScrapeMetrics(_ context.Context) (pmetric.Metrics, error) {
	return m.metrics, nil
}

func (m *mockScraper) Shutdown(_ context.Context) error {
	m.stopped = true
	return nil
}

func TestResourceAttributeScraper_HostLevel(t *testing.T) {
	// Create metrics with one ResourceMetrics (typical for host-level scrapers)
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("system.cpu.time")

	mock := &mockScraper{metrics: metrics}

	scraper := &resourceAttributeScraper{
		delegate:       mock,
		baseInstanceID: generateServiceInstanceID("testhost"),
		scraperType:    component.MustNewType("cpu"),
	}

	// Test Start
	err := scraper.Start(t.Context(), nil)
	require.NoError(t, err)
	assert.True(t, mock.started)

	// Test ScrapeMetrics
	result, err := scraper.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	// Verify resource attributes were injected
	require.Equal(t, 1, result.ResourceMetrics().Len())
	attrs := result.ResourceMetrics().At(0).Resource().Attributes()

	instanceID, ok := attrs.Get("service.instance.id")
	require.True(t, ok, "service.instance.id should be set")
	assert.Equal(t, scraper.baseInstanceID, instanceID.Str())

	// Test Shutdown
	err = scraper.Shutdown(t.Context())
	require.NoError(t, err)
	assert.True(t, mock.stopped)
}

func TestResourceAttributeScraper_ProcessLevel(t *testing.T) {
	// Create metrics with multiple ResourceMetrics (typical for process scraper)
	metrics := pmetric.NewMetrics()

	// Process 1
	rm1 := metrics.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutInt("process.pid", 1234)
	rm1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("process.cpu.time")

	// Process 2
	rm2 := metrics.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutInt("process.pid", 5678)
	rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("process.cpu.time")

	mock := &mockScraper{metrics: metrics}
	baseInstanceID := generateServiceInstanceID("testhost")

	scraper := &resourceAttributeScraper{
		delegate:       mock,
		baseInstanceID: baseInstanceID,
		scraperType:    component.MustNewType("process"),
	}

	result, err := scraper.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	require.Equal(t, 2, result.ResourceMetrics().Len())

	// Verify first process
	attrs1 := result.ResourceMetrics().At(0).Resource().Attributes()
	instanceID1, ok := attrs1.Get("service.instance.id")
	require.True(t, ok)
	expectedID1 := generateProcessServiceInstanceID(baseInstanceID, 1234)
	assert.Equal(t, expectedID1, instanceID1.Str())

	// Verify second process
	attrs2 := result.ResourceMetrics().At(1).Resource().Attributes()
	instanceID2, ok := attrs2.Get("service.instance.id")
	require.True(t, ok)
	expectedID2 := generateProcessServiceInstanceID(baseInstanceID, 5678)
	assert.Equal(t, expectedID2, instanceID2.Str())

	// Verify the two processes have different instance IDs
	assert.NotEqual(t, instanceID1.Str(), instanceID2.Str(),
		"different processes should have different service.instance.id")
}

func TestResourceAttributeScraper_ProcessWithoutPID(t *testing.T) {
	// Edge case: process scraper returns ResourceMetrics without process.pid
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("process.cpu.time")
	// Note: no process.pid attribute set

	mock := &mockScraper{metrics: metrics}
	baseInstanceID := generateServiceInstanceID("testhost")

	scraper := &resourceAttributeScraper{
		delegate:       mock,
		baseInstanceID: baseInstanceID,
		scraperType:    component.MustNewType("process"),
	}

	result, err := scraper.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	// Should fall back to base instance ID
	attrs := result.ResourceMetrics().At(0).Resource().Attributes()
	instanceID, ok := attrs.Get("service.instance.id")
	require.True(t, ok)
	assert.Equal(t, baseInstanceID, instanceID.Str(),
		"should fall back to base instance ID when process.pid is missing")
}

func TestResourceAttributeScraper_Disabled(t *testing.T) {
	// Verify that when the resourceAttributeScraper wrapper is NOT applied,
	// metrics pass through without service.instance.id being set.
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("system.cpu.time")

	mock := &mockScraper{metrics: metrics}

	// Scrape directly from the mock (no resourceAttributeScraper wrapper)
	result, err := mock.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	require.Equal(t, 1, result.ResourceMetrics().Len())
	attrs := result.ResourceMetrics().At(0).Resource().Attributes()
	_, ok := attrs.Get("service.instance.id")
	assert.False(t, ok, "service.instance.id should not be set when wrapper is not applied")
}

// mockScraperWithError is a test scraper that returns both metrics and an error
type mockScraperWithError struct {
	metrics pmetric.Metrics
	err     error
}

func (*mockScraperWithError) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (m *mockScraperWithError) ScrapeMetrics(_ context.Context) (pmetric.Metrics, error) {
	return m.metrics, m.err
}

func (*mockScraperWithError) Shutdown(_ context.Context) error {
	return nil
}

func TestResourceAttributeScraper_PartialError(t *testing.T) {
	// Test that resource attributes are injected even when there's a partial error
	// This is a common scenario in the hostmetricsreceiver
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("system.cpu.time")

	partialErr := assert.AnError // Simulate a partial scrape error

	mock := &mockScraperWithError{metrics: metrics, err: partialErr}

	scraper := &resourceAttributeScraper{
		delegate:       mock,
		baseInstanceID: generateServiceInstanceID("testhost"),
		scraperType:    component.MustNewType("cpu"),
	}

	result, err := scraper.ScrapeMetrics(t.Context())

	// Should return the original error
	assert.ErrorIs(t, err, partialErr)

	// But resource attributes should still be injected
	require.Equal(t, 1, result.ResourceMetrics().Len())
	attrs := result.ResourceMetrics().At(0).Resource().Attributes()

	_, ok := attrs.Get("service.instance.id")
	require.True(t, ok, "service.instance.id should be set even on partial error")
}
