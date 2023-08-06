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

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestScraper(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		filename string
		state    bool
		addr     *confignet.NetAddr
	}{
		{
			name:     "tcp up",
			filename: "metrics_tcp.json",
			state:    true,
			addr: &confignet.NetAddr{
				Endpoint:  "localhost:44321",
				Transport: "tcp",
			},
		},
		{
			name:     "unix up",
			state:    true,
			filename: "metrics_unix.json",
			addr: &confignet.NetAddr{
				Endpoint:  "localhost:44321",
				Transport: "unix",
			},
		},
		{
			name:     "tcp down",
			filename: "metrics_tcp_down.json",
			state:    false,
			addr: &confignet.NetAddr{
				Endpoint:  "localhost:44321",
				Transport: "tcp",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// *** setup server and accepting connections
			// use state variable to don't listen on connection to simulate a failure case
			if tc.state {
				l, err := tc.addr.Listen()
				require.NoError(t, err, "creating listener")
				defer l.Close()

				go func() {
					for {
						conn, tErr := l.Accept()
						if tErr != nil {
							break
						}

						_, tErr = io.Copy(io.Discard, conn)
						if tErr != nil {
							break
						}
					}
				}()
			}
			// *** end server

			expectedFile := filepath.Join("testdata", "expected_metrics", tc.filename)
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)

			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.CollectionInterval = 100 * time.Millisecond
			cfg.Endpoint = tc.addr.Endpoint
			if len(cfg.Transport) > 0 {
				cfg.Transport = tc.addr.Transport
			}

			settings := receivertest.NewNopCreateSettings()

			scrpr := newScraper(cfg, settings)
			require.NoError(t, scrpr.start(context.Background(), componenttest.NewNopHost()), "failed starting scraper")

			ctx, done := context.WithTimeout(context.Background(), 100*time.Millisecond)
			actualMetrics, err := scrpr.scrape(ctx)
			require.NoError(t, err, "failed scrape")
			done()

			require.NoError(
				t,
				pmetrictest.CompareMetrics(
					expectedMetrics,
					actualMetrics,
					pmetrictest.IgnoreTimestamp(),
					pmetrictest.IgnoreStartTimestamp(),
				),
			)
		})
	}
}

func TestTimeout(t *testing.T) {
	t.Parallel()
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
