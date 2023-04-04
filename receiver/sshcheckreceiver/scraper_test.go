// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

type opensshContainer struct {
	testcontainers.Container
	Endpoint string
}

func setupSSHServer(t *testing.T) *opensshContainer {
	wd, err := os.Getwd()
	require.NoError(t, err)
	req := testcontainers.ContainerRequest{
		Image: "linuxserver/openssh-server:version-8.8_p1-r1",
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      wd + "/testdata/config/sshd_config",
				ContainerFilePath: "/config/sshd_config",
			},
		},
		Env: map[string]string{
			"USER_NAME":       "otelu",
			"USER_PASSWORD":   "otelp",
			"PASSWORD_ACCESS": "true",
		},
		ExposedPorts: []string{"2222/tcp"},
		WaitingFor:   wait.ForListeningPort("2222/tcp"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	require.NotNil(t, container)

	mappedPort, err := container.MappedPort(ctx, "2222")
	require.NoError(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	endpoint := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

	t.Log("endpoint: ", endpoint)
	return &opensshContainer{Container: container, Endpoint: endpoint}
}

func TestScraper(t *testing.T) {
	t.Skip("Skipping as the test fails intermittently, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/19473")
	if !supportedOS() {
		t.Skip("Skip tests if not running on one of: [linux, darwin, freebsd, openbsd]")
	}
	c := setupSSHServer(t)
	defer func() {
		require.NoError(t, c.Terminate(context.Background()), "terminating container")
	}()
	require.NotEmpty(t, c.Endpoint)

	testCases := []struct {
		name       string
		filename   string
		enableSFTP bool
	}{
		{
			name:     "metrics_golden",
			filename: "metrics_golden.yaml",
		},
		{
			name:       "metrics_golden_sftp",
			filename:   "metrics_golden_sftp.yaml",
			enableSFTP: true,
		},
		{
			name:     "cannot_authenticate",
			filename: "cannot_authenticate.yaml",
		},
		{
			name:     "invalid_endpoint",
			filename: "invalid_endpoint.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedFile := filepath.Join("testdata", "expected_metrics", tc.filename)
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)

			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
			cfg.Username = "otelu"
			cfg.Password = "otelp"
			cfg.Endpoint = c.Endpoint
			cfg.IgnoreHostKey = true
			if tc.enableSFTP {
				cfg.MetricsBuilderConfig.Metrics.SshcheckSftpStatus.Enabled = true
				cfg.MetricsBuilderConfig.Metrics.SshcheckSftpDuration.Enabled = true
			}

			settings := receivertest.NewNopCreateSettings()

			scrpr := newScraper(cfg, settings)
			require.NoError(t, scrpr.start(context.Background(), componenttest.NewNopHost()), "failed starting scraper")

			actualMetrics, err := scrpr.scrape(context.Background())
			require.NoError(t, err, "failed scrape")
			require.NoError(
				t,
				pmetrictest.CompareMetrics(
					expectedMetrics,
					actualMetrics,
					pmetrictest.IgnoreMetricValues("sshcheck.duration", "sshcheck.sftp_duration"),
					pmetrictest.IgnoreTimestamp(),
					pmetrictest.IgnoreStartTimestamp(),
					pmetrictest.IgnoreMetricAttributeValue("sshcheck", "endpoint"),
				),
			)
		})
	}
}

func TestScraperDoesNotErrForSSHErr(t *testing.T) {
	if !supportedOS() {
		t.Skip("Skip tests if not running on one of: [linux, darwin, freebsd, openbsd]")
	}
	c := setupSSHServer(t)
	defer func() {
		require.NoError(t, c.Terminate(context.Background()), "terminating container")
	}()
	require.NotEmpty(t, c.Endpoint)

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
	cfg.Username = "not-the-user"
	cfg.Password = "not-the-password"
	cfg.Endpoint = c.Endpoint
	cfg.IgnoreHostKey = true

	settings := receivertest.NewNopCreateSettings()

	scraper := newScraper(cfg, settings)
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()), "should not err to start")

	_, err := scraper.scrape(context.Background())
	require.NoError(t, err, "should not err")
}

func TestTimeout(t *testing.T) {
	if !supportedOS() {
		t.Skip("Skip tests if not running on one of: [linux, darwin, freebsd, openbsd]")
	}
	testCases := []struct {
		name     string
		deadline time.Time
		timeout  time.Duration
		want     time.Duration
	}{
		{
			name:     "timeout is shorter",
			deadline: time.Now().Add(time.Second),
			timeout:  time.Second * 2,
			want:     time.Second,
		},
		{
			name:     "deadline is shorter",
			deadline: time.Now().Add(time.Second * 2),
			timeout:  time.Second,
			want:     time.Second,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			to := timeout(tc.deadline, tc.timeout)
			if to < (tc.want-10*time.Millisecond) || to > tc.want {
				t.Fatalf("wanted time within 10 milliseconds: %s, got: %s", time.Second, to)
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond

	settings := receivertest.NewNopCreateSettings()

	scrpr := newScraper(cfg, settings)
	if !supportedOS() {
		require.Error(t, scrpr.start(context.Background(), componenttest.NewNopHost()), "should err starting scraper")
		return
	}
	require.NoError(t, scrpr.start(context.Background(), componenttest.NewNopHost()), "failed starting scraper")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scrpr.scrape(ctx)
	require.Error(t, err, "should have returned error on canceled context")
	require.EqualValues(t, err.Error(), ctx.Err().Error(), "scrape should return context's error")

}

// issue # 18193
// init failures resulted in scrape panic for SFTP client
func TestWithoutStartErrsNotPanics(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
	cfg.Username = "otelu"
	cfg.Password = "otelp"
	cfg.Endpoint = "localhost:22"
	cfg.IgnoreHostKey = true
	cfg.MetricsBuilderConfig.Metrics.SshcheckSftpStatus.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SshcheckSftpDuration.Enabled = true

	// create the scraper without starting it, so Client is nil
	scrpr := newScraper(cfg, receivertest.NewNopCreateSettings())

	// scrape should error not panic
	var err error
	require.NotPanics(t, func() { _, err = scrpr.scrape(context.Background()) }, "scrape should not panic")
	require.Error(t, err, "expected scrape to err when without start")
}
