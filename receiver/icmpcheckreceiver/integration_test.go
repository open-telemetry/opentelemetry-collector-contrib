// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package icmpcheckreceiver

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func setupTest(t *testing.T) {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("skipping test on non-Linux OS")
		return
	}

	if os.Geteuid() != 0 {
		t.Skip("skipping test on non-root user")
		return
	}

	if err := exec.Command("sudo", "sysctl", "-w", "net.ipv4.ping_group_range=0 2147483647").Run(); err != nil {
		t.Skip("skipping test as setting ping_group_range failed:", err)
	}
}

func TestIntegrationWithSudo(t *testing.T) {
	setupTest(t)

	var containerIP atomic.Value
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image: "alpine",
				Cmd:   []string{"sleep", "300"},
				LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
					PostReadies: []testcontainers.ContainerHook{
						func(ctx context.Context, container testcontainers.Container) error {
							ip, err := container.ContainerIP(ctx)
							if err != nil {
								return err
							}
							containerIP.Store(ip)
							return nil
						},
					},
				}},
			},
		),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				hostIP := containerIP.Load().(string)
				if hostIP == "" {
					t.Error("Failed to get container IP")
				} else {
					rCfg := cfg.(*Config)
					rCfg.CollectionInterval = 100 * time.Millisecond
					rCfg.Targets = []PingTarget{
						{Host: hostIP, PingCount: 4},
					}
				}
			},
		),
		scraperinttest.WithCompareTimeout(30*time.Second),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues("ping.rtt.max", "ping.rtt.min", "ping.rtt.avg", "ping.rtt.stddev"),
			pmetrictest.IgnoreResourceAttributeValue("net.peer.name"),
			pmetrictest.IgnoreResourceAttributeValue("net.peer.ip"),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
