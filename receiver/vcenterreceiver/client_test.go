// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	vsantypes "github.com/vmware/govmomi/vsan/types"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

func TestDatacenters(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			vm:        vm,
		}
		dcs, err := client.Datacenters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, dcs)
	})
}

func TestDatastores(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		dss, err := client.Datastores(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, dss)
	})
}

func TestEmptyDatastores(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datastore = 0
	vpx.Machine = 0
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		dss, err := client.Datastores(ctx, dc.Reference())
		require.NoError(t, err)
		require.Empty(t, dss)
	}, vpx)
}

func TestComputeResources(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		crs, err := client.ComputeResources(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, crs)
	})
}

func TestComputeResourcesWithStandalone(t *testing.T) {
	esx := simulator.ESX()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		crs, err := client.ComputeResources(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, crs)
	}, esx)
}

func TestHostSystems(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		hss, err := client.HostSystems(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, hss)
	})
}

func TestEmptyHostSystems(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Host = 0
	vpx.ClusterHost = 0
	vpx.Machine = 0
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		hss, err := client.HostSystems(ctx, dc.Reference())
		require.NoError(t, err)
		require.Empty(t, hss)
	}, vpx)
}

func TestResourcePools(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		rps, err := client.ResourcePools(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, rps)
	})
}

func TestVMs(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		vms, err := client.VMs(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, vms)
	})
}

func TestEmptyVMs(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Machine = 0
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		vms, err := client.VMs(ctx, dc.Reference())
		require.NoError(t, err)
		require.Empty(t, vms)
	}, vpx)
}

func TestPerfMetricsQuery(t *testing.T) {
	esx := simulator.ESX()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pm := performance.NewManager(c)
		m := view.NewManager(c)
		finder := find.NewFinder(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			vm:        m,
			pm:        pm,
			finder:    finder,
		}
		hs, err := finder.DefaultHostSystem(ctx)
		require.NoError(t, err)

		spec := types.PerfQuerySpec{Format: string(types.PerfFormatNormal), IntervalId: int32(20)}
		metrics, err := client.PerfMetricsQuery(ctx, spec, hostPerfMetricList, []types.ManagedObjectReference{hs.Reference()})
		require.NoError(t, err)
		require.NotEmpty(t, metrics.resultsByRef)
	}, esx)
}

func TestPerfMetricsQuery_FallbackOnBatchError(t *testing.T) {
	esx := simulator.ESX()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pm := performance.NewManager(c)
		m := view.NewManager(c)
		finder := find.NewFinder(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			vm:        m,
			pm:        pm,
			finder:    finder,
		}
		hs, err := finder.DefaultHostSystem(ctx)
		require.NoError(t, err)

		spec := types.PerfQuerySpec{Format: string(types.PerfFormatNormal), IntervalId: int32(20)}

		metrics, err := client.PerfMetricsQuery(ctx, spec, []string{"invalid.metric.name"}, []types.ManagedObjectReference{hs.Reference()})
		require.NoError(t, err)
		require.NotNil(t, metrics)
		require.Empty(t, metrics.resultsByRef)
	}, esx)
}

func TestPerfMetricsQueryBatching(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Host = 10
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pm := performance.NewManager(c)
		m := view.NewManager(c)
		finder := find.NewFinder(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			vm:        m,
			pm:        pm,
			finder:    finder,
			cfg: &Config{
				MaxQueryMetrics: len(hostPerfMetricList) * 3,
			},
		}

		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)

		hss, err := client.HostSystems(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, hss)

		var refs []types.ManagedObjectReference
		for _, hs := range hss {
			refs = append(refs, hs.Reference())
		}

		spec := types.PerfQuerySpec{Format: string(types.PerfFormatNormal), IntervalId: int32(20)}
		metrics, err := client.PerfMetricsQuery(ctx, spec, hostPerfMetricList, refs)
		require.NoError(t, err)
		require.Len(t, metrics.resultsByRef, len(hss))
	}, vpx)
}

func TestDatacenterInventoryListObjects(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := client.DatacenterInventoryListObjects(ctx)
		require.NoError(t, err)
		require.Len(t, dcs, 2)
	}, vpx)
}

func TestResourcePoolInventoryListObjects(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := finder.DatacenterList(ctx, "*")
		require.NoError(t, err)
		rps, err := client.ResourcePoolInventoryListObjects(ctx, dcs)
		require.NoError(t, err)
		require.NotEmpty(t, rps)
	}, vpx)
}

func TestVAppInventoryListObjects(t *testing.T) {
	// Currently skipping as the Simulator has no vApps by default and setting
	// vApps appears to be broken
	t.Skip()
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	vpx.App = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := finder.DatacenterList(ctx, "*")
		require.NoError(t, err)
		vApps, err := client.VAppInventoryListObjects(ctx, dcs)
		require.NoError(t, err)
		require.NotEmpty(t, vApps)
	}, vpx)
}

func TestEmptyVAppInventoryListObjects(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := finder.DatacenterList(ctx, "*")
		require.NoError(t, err)
		vApps, err := client.VAppInventoryListObjects(ctx, dcs)
		require.NoError(t, err)
		require.Empty(t, vApps)
	}, vpx)
}

func TestSessionReestablish(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		sm := session.NewManager(c)
		pw, _ := simulator.DefaultLogin.Password()
		client := vcenterClient{
			logger:    zap.NewNop(),
			vimDriver: c,
			cfg: &Config{
				Username: simulator.DefaultLogin.Username(),
				Password: configopaque.String(pw),
				Endpoint: fmt.Sprintf("%s://%s", c.URL().Scheme, c.URL().Host),
				ClientConfig: configtls.ClientConfig{
					Insecure: true,
				},
			},
			sessionManager: sm,
		}
		err := sm.Logout(ctx)
		require.NoError(t, err)

		connected, err := client.sessionManager.SessionIsActive(ctx)
		require.NoError(t, err)
		require.False(t, connected)

		err = client.EnsureConnection(ctx)
		require.NoError(t, err)

		connected, err = client.sessionManager.SessionIsActive(ctx)
		require.NoError(t, err)
		require.True(t, connected)
	})
}

func TestApplyProxy(t *testing.T) {
	tests := []struct {
		desc          string
		proxyURL      string
		expectedProxy string
		expectedErr   string
	}{
		{
			desc:     "unset leaves the transport alone",
			proxyURL: "",
		},
		{
			desc:          "http proxy",
			proxyURL:      "http://proxy.some-host:8080",
			expectedProxy: "http://proxy.some-host:8080",
		},
		{
			desc:          "https proxy",
			proxyURL:      "https://proxy.some-host:8443",
			expectedProxy: "https://proxy.some-host:8443",
		},
		{
			desc:          "socks5 proxy",
			proxyURL:      "socks5://proxy.some-host:1080",
			expectedProxy: "socks5://proxy.some-host:1080",
		},
		{
			desc:          "socks5h proxy",
			proxyURL:      "socks5h://proxy.some-host:1080",
			expectedProxy: "socks5h://proxy.some-host:1080",
		},
		{
			desc:        "unparsable proxy",
			proxyURL:    "h" + string(rune(0x7f)),
			expectedErr: "unable to parse proxy_url",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			transport := &http.Transport{}
			err := applyProxy(transport, tc.proxyURL)

			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				require.Nil(t, transport.Proxy)
				return
			}
			require.NoError(t, err)

			if tc.expectedProxy == "" {
				require.Nil(t, transport.Proxy)
				return
			}

			require.NotNil(t, transport.Proxy)
			proxyURL, err := transport.Proxy(&http.Request{
				URL: &url.URL{Scheme: "https", Host: "vcsa.some-host"},
			})
			require.NoError(t, err)
			require.Equal(t, tc.expectedProxy, proxyURL.String())
		})
	}
}

// net/http hands DialTLSContext the proxy's address when the proxy itself
// speaks https, and govmomi's hook would verify the vCenter tls settings
// against the proxy's certificate, so applyProxy has to clear it.
func TestApplyProxyClearsCustomTLSDialer(t *testing.T) {
	dialer := func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("should not be called")
	}

	t.Run("cleared when a proxy is set", func(t *testing.T) {
		transport := &http.Transport{DialTLSContext: dialer}
		require.NoError(t, applyProxy(transport, "https://proxy.some-host:8443"))
		require.Nil(t, transport.DialTLSContext)
	})

	t.Run("kept when no proxy is set", func(t *testing.T) {
		transport := &http.Transport{DialTLSContext: dialer}
		require.NoError(t, applyProxy(transport, ""))
		require.NotNil(t, transport.DialTLSContext)
	})
}

// newPlainHTTPSimulator starts a vcsim instance without TLS so that tests can
// reach it through a plain HTTP forward proxy. simulator.Test and Model.Run
// always force TLS, so the model is built up by hand here.
func newPlainHTTPSimulator(t *testing.T) *simulator.Server {
	t.Helper()

	model := simulator.VPX()
	require.NoError(t, model.Create())
	t.Cleanup(model.Remove)

	server := model.Service.NewServer()
	t.Cleanup(server.Close)

	require.Equal(t, "http", server.URL.Scheme)

	return server
}

func simulatorConfig(server *simulator.Server, proxyURL string) *Config {
	pw, _ := simulator.DefaultLogin.Password()

	return &Config{
		Username: simulator.DefaultLogin.Username(),
		Password: configopaque.String(pw),
		Endpoint: fmt.Sprintf("%s://%s", server.URL.Scheme, server.URL.Host),
		ProxyURL: proxyURL,
		ClientConfig: configtls.ClientConfig{
			Insecure: true,
		},
	}
}

func TestEnsureConnectionThroughProxy(t *testing.T) {
	server := newPlainHTTPSimulator(t)

	var proxied atomic.Int64
	reverseProxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: server.URL.Scheme,
		Host:   server.URL.Host,
	})
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Add(1)
		reverseProxy.ServeHTTP(w, r)
	}))
	defer proxy.Close()

	client := vcenterClient{
		logger: zap.NewNop(),
		cfg:    simulatorConfig(server, proxy.URL),
	}

	ctx := t.Context()
	require.NoError(t, client.EnsureConnection(ctx))
	defer func() {
		require.NoError(t, client.Disconnect(ctx))
	}()

	connected, err := client.sessionManager.SessionIsActive(ctx)
	require.NoError(t, err)
	require.True(t, connected)

	// The SDK endpoint is only reachable here if the request actually went
	// through the proxy, but assert it explicitly so a silently ignored
	// proxy_url cannot pass this test.
	require.Positive(t, proxied.Load())
}

func TestEnsureConnectionUnreachableProxy(t *testing.T) {
	server := newPlainHTTPSimulator(t)

	// Take a real address and immediately release it so nothing is listening.
	closedProxy := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedProxyURL := closedProxy.URL
	closedProxy.Close()

	client := vcenterClient{
		logger: zap.NewNop(),
		cfg:    simulatorConfig(server, closedProxyURL),
	}

	require.Error(t, client.EnsureConnection(t.Context()))
}

func TestConvertVSANResultToMetricResults(t *testing.T) {
	client := vcenterClient{logger: zap.NewNop()}

	tests := []struct {
		name         string
		result       vsantypes.VsanPerfEntityMetricCSV
		expectedUUID string
	}{
		{
			name: "empty SampleInfo is skipped without error",
			result: vsantypes.VsanPerfEntityMetricCSV{
				EntityRefId: "cluster-domclient:52e0c79c-1111-2222-3333-444455556666",
				SampleInfo:  "",
				Value: []vsantypes.VsanPerfMetricSeriesCSV{
					{
						MetricId: vsantypes.VsanPerfMetricId{Label: "iopsRead"},
						Values:   "",
					},
				},
			},
			expectedUUID: "52e0c79c-1111-2222-3333-444455556666",
		},
		{
			name: "whitespace-only SampleInfo is skipped without error",
			result: vsantypes.VsanPerfEntityMetricCSV{
				EntityRefId: "host-domclient:aaaabbbb-cccc-dddd-eeee-ffff00001111",
				SampleInfo:  "  ",
			},
			expectedUUID: "aaaabbbb-cccc-dddd-eeee-ffff00001111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricResults, err := client.convertVSANResultToMetricResults(tt.result)
			require.NoError(t, err)
			require.NotNil(t, metricResults)
			require.Equal(t, tt.expectedUUID, metricResults.UUID)
			require.Empty(t, metricResults.MetricDetails)
		})
	}
}
