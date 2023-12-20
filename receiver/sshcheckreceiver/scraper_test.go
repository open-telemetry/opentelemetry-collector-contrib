// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"golang.org/x/crypto/ssh"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func setupSSHServer(t *testing.T) string {
	config := &ssh.ServerConfig{
		NoClientAuth: true,
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "otelu" && string(pass) == "otelp" {
				return nil, nil
			}
			return nil, fmt.Errorf("wrong username or password")
		},
	}

	privateBytes, err := os.ReadFile("testdata/keys/id_rsa")
	require.NoError(t, err)

	private, err := ssh.ParsePrivateKey(privateBytes)
	require.NoError(t, err)

	config.AddHostKey(private)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				break
			}
			_, chans, reqs, err := ssh.NewServerConn(conn, config)
			if err != nil {
				t.Logf("Failed to handshake: %v", err)
				continue
			}
			go ssh.DiscardRequests(reqs)
			go handleChannels(chans)
		}
	}()

	return listener.Addr().String()
}

func handleChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		if t := newChannel.ChannelType(); t != "session" {
			if err := newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t)); err != nil {
				return
			}
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				if req.Type == "subsystem" && string(req.Payload[4:]) == "sftp" {
					ok = true
					go func() {
						defer channel.Close()

						server := sftp.NewRequestServer(channel, sftp.Handlers{
							FileGet:  sftp.InMemHandler().FileGet,
							FilePut:  sftp.InMemHandler().FilePut,
							FileCmd:  sftp.InMemHandler().FileCmd,
							FileList: sftp.InMemHandler().FileList,
						})
						if err != nil {
							return
						}

						if err := server.Serve(); errors.Is(err, io.EOF) {
							server.Close()
						} else if err != nil {
							return
						}
					}()
				}
				if err := req.Reply(ok, nil); err != nil {
					return
				}
			}
		}(requests)
	}
}

func TestScraper(t *testing.T) {
	if !supportedOS() {
		t.Skip("Skip tests if not running on one of: [linux, darwin, freebsd, openbsd]")
	}
	endpoint := setupSSHServer(t)
	require.NotEmpty(t, endpoint)

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
			cfg.Endpoint = endpoint
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

func TestScraperPropagatesResourceAttributes(t *testing.T) {
	if !supportedOS() {
		t.Skip("Skip tests if not running on one of: [linux, darwin, freebsd, openbsd]")
	}
	endpoint := setupSSHServer(t)
	require.NotEmpty(t, endpoint)

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.MetricsBuilderConfig.ResourceAttributes.SSHEndpoint.Enabled = true
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
	cfg.Username = "otelu"
	cfg.Password = "otelp"
	cfg.Endpoint = endpoint
	cfg.IgnoreHostKey = true

	settings := receivertest.NewNopCreateSettings()

	scraper := newScraper(cfg, settings)
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()), "failed starting scraper")

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err, "failed scrape")

	resourceMetrics := actualMetrics.ResourceMetrics()
	expectedResourceAttributes := map[string]any{"ssh.endpoint": endpoint}
	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceAttributes := resourceMetrics.At(i).Resource().Attributes()
		for name, value := range expectedResourceAttributes {
			actualAttributeValue, ok := resourceAttributes.Get(name)
			require.True(t, ok)
			require.Equal(t, value, actualAttributeValue.Str())
		}
	}
}

func TestScraperDoesNotErrForSSHErr(t *testing.T) {
	if !supportedOS() {
		t.Skip("Skip tests if not running on one of: [linux, darwin, freebsd, openbsd]")
	}
	endpoint := setupSSHServer(t)
	require.NotEmpty(t, endpoint)

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
	cfg.Username = "not-the-user"
	cfg.Password = "not-the-password"
	cfg.Endpoint = endpoint
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
