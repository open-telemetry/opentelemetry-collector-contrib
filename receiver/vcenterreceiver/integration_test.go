// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	model := simulator.VPX()

	model.Host = 2
	model.Machine = 2

	err := model.Create()
	require.NoError(t, err)

	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)

		hosts, err := finder.HostSystemList(ctx, "/DC0/host/*")
		require.NoError(t, err)
		for i, host := range hosts {
			newName := fmt.Sprintf("H%d", i)
			simulator.Map(ctx).Get(host.Reference()).(*simulator.HostSystem).Name = newName
		}

		vms, err := finder.VirtualMachineList(ctx, "/DC0/vm/*")
		require.NoError(t, err)
		for i, vm := range vms {
			newName := fmt.Sprintf("VM%d", i)
			newUUID := fmt.Sprintf("vm-uuid-%d", i)
			simVM := simulator.Map(ctx).Get(vm.Reference()).(*simulator.VirtualMachine)
			simVM.Name = newName
			simVM.Config.Uuid = newUUID

			// Explicitly assign VM to a specific host
			hostIndex := i % len(hosts) // This will alternate VMs between hosts
			host := hosts[hostIndex]
			hostRef := host.Reference()
			relocateSpec := types.VirtualMachineRelocateSpec{
				Host: &hostRef,
			}
			task, err := vm.Relocate(ctx, relocateSpec, types.VirtualMachineMovePriorityDefaultPriority)
			require.NoError(t, err)
			require.NoError(t, task.Wait(ctx))

			// Reconfigure the VM to apply name and UUID changes
			task, err = vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{
				Name: newName,
				Uuid: newUUID,
			})
			require.NoError(t, err)
			require.NoError(t, task.Wait(ctx))
		}
		pw, set := simulator.DefaultLogin.Password()
		require.True(t, set)

		s := session.NewManager(c)
		newVcenterClient = func(l *zap.Logger, cfg *Config) *vcenterClient {
			client := &vcenterClient{
				logger:         l,
				cfg:            cfg,
				sessionManager: s,
				vimDriver:      c,
			}
			require.NoError(t, client.EnsureConnection(context.Background()))
			client.vimDriver = c
			client.finder = find.NewFinder(c)
			// Performance/vSAN metrics rely on time based publishing so this is inherently flaky for an
			// integration test, so setting the performance manager to nil to not attempt to compare
			// performance metrics. Coverage for this is encompassed in ./scraper_test.go
			client.pm = nil
			client.vsanDriver = nil
			return client
		}
		defer func() {
			newVcenterClient = defaultNewVcenterClient
		}()
		scraperinttest.NewIntegrationTest(
			NewFactory(),
			scraperinttest.WithCustomConfig(
				func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
					rCfg := cfg.(*Config)
					rCfg.CollectionInterval = 2 * time.Second
					rCfg.Endpoint = fmt.Sprintf("%s://%s", c.URL().Scheme, c.URL().Host)
					rCfg.Username = simulator.DefaultLogin.Username()
					rCfg.Password = configopaque.String(pw)
					rCfg.ClientConfig = configtls.ClientConfig{
						Insecure: true,
					}
				}),
			scraperinttest.WithCompareOptions(
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreDatapointAttributesOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreMetricValues(),
			),
		).Run(t)
	})
}
