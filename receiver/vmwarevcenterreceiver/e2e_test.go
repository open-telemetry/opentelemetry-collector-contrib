// Copyright  The OpenTelemetry Authors
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

package vmwarevcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

func TestEndtoEnd_ESX(t *testing.T) {
	sim := simulator.ESX()
	defer sim.Remove()

	sim.Run(func(ctx context.Context, c *vim25.Client) error {
		cfg := &Config{
			MetricsConfig: &MetricsConfig{
				TLSClientSetting: configtls.TLSClientSetting{
					Insecure: true,
				},
			},
		}
		s := session.NewManager(c)

		scraper := newVmwareVcenterScraper(zap.NewNop(), cfg)
		scraper.client.moClient = &govmomi.Client{
			Client:         c,
			SessionManager: s,
		}
		rcvr := &vcenterReceiver{
			config:  cfg,
			scraper: scraper,
		}

		err := rcvr.Start(ctx, componenttest.NewNopHost())
		require.NoError(t, err)

		err = rcvr.Shutdown(ctx)
		require.NoError(t, err)
		return nil
	})
}
