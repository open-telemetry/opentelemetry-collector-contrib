// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestatsreceiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func Test_Scrape(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newDefaultConfig().(*Config)
	cfg.Include = filepath.Join(tmpDir, "*.log")
	s := newScraper(cfg, receivertest.NewNopCreateSettings())
	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, metrics.ResourceMetrics().Len())
	logFile := filepath.Join(tmpDir, "my.log")
	err = os.WriteFile(logFile, []byte("something"), 0600)
	t.Cleanup(func() {
		_ = os.Remove(tmpDir)
	})
	require.NoError(t, err)
	fileinfo, err := os.Stat(logFile)
	require.NoError(t, err)
	require.Equal(t, int64(9), fileinfo.Size())
	matches, err := doublestar.FilepathGlob(cfg.Include)
	require.NoError(t, err)
	require.Equal(t, []string{logFile}, matches)
	metrics, err = s.scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, 2, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	mTimeMetric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, "file.mtime", mTimeMetric.Name())
	require.Equal(t, fileinfo.ModTime().Unix(), mTimeMetric.Sum().DataPoints().At(0).IntValue())
	sizeMetric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1)
	require.Equal(t, "file.size", sizeMetric.Name())
	require.Equal(t, int64(9), sizeMetric.Gauge().DataPoints().At(0).IntValue())
}

func Test_Scrape_All(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newDefaultConfig().(*Config)
	cfg.Include = filepath.Join(tmpDir, "*.log")
	cfg.Metrics.FileAtime.Enabled = true
	cfg.Metrics.FileCtime.Enabled = true

	s := newScraper(cfg, receivertest.NewNopCreateSettings())
	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, metrics.ResourceMetrics().Len())
	logFile := filepath.Join(tmpDir, "my.log")
	err = os.WriteFile(logFile, []byte("something"), 0600)
	t.Cleanup(func() {
		_ = os.Remove(tmpDir)
	})
	require.NoError(t, err)
	fileinfo, err := os.Stat(logFile)
	require.NoError(t, err)
	require.Equal(t, int64(9), fileinfo.Size())
	matches, err := doublestar.FilepathGlob(cfg.Include)
	require.NoError(t, err)
	require.Equal(t, []string{logFile}, matches)
	metrics, err = s.scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, 4, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	aTimeMetric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, "file.atime", aTimeMetric.Name())
	require.Equal(t, fileinfo.ModTime().Unix(), aTimeMetric.Sum().DataPoints().At(0).IntValue())
	cTimeMetric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1)
	require.Equal(t, "file.ctime", cTimeMetric.Name())
	require.Equal(t, fileinfo.ModTime().Unix(), cTimeMetric.Sum().DataPoints().At(0).IntValue())
	mTimeMetric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2)
	require.Equal(t, "file.mtime", mTimeMetric.Name())
	require.Equal(t, fileinfo.ModTime().Unix(), mTimeMetric.Sum().DataPoints().At(0).IntValue())
	sizeMetric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3)
	require.Equal(t, "file.size", sizeMetric.Name())
	require.Equal(t, int64(9), sizeMetric.Gauge().DataPoints().At(0).IntValue())
}
