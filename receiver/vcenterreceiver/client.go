// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	vt "github.com/vmware/govmomi/vim25/types"
	"go.uber.org/zap"
)

type client interface {
	// EnsureConnection will establish a connection to the vSphere SDK if not already established
	EnsureConnection(ctx context.Context, logger *zap.Logger) error
	// Datacenters returns the datacenterComputeResources of the vSphere SDK
	Datacenters(ctx context.Context) ([]*object.Datacenter, error)
	// Clusters returns the clusterComputeResources of the vSphere SDK
	Clusters(ctx context.Context, datacenter *object.Datacenter) ([]*object.ClusterComputeResource, error)
	// ResourcePools returns the resourcePools in the vSphere SDK
	ResourcePools(ctx context.Context) ([]*object.ResourcePool, error)
	VMs(ctx context.Context) ([]*object.VirtualMachine, error)
	Disconnect(ctx context.Context) error

	PerformanceQuery(
		ctx context.Context,
		spec vt.PerfQuerySpec,
		names []string,
		objs []vt.ManagedObjectReference,
	) (*perfSampleResult, error)
}

func newCachingClient(c *Config) client {
	cache := ttlcache.NewCache()
	_ = cache.SetTTL(c.CachingTTL)
	cache.SkipTTLExtensionOnHit(true)
	return &cachingClient{
		delegate:   defaultNewVcenterClient(c),
		cache:      cache,
		refreshTTL: c.RefreshTTL,
	}
}

type cachingClient struct {
	delegate    client
	cache       *ttlcache.Cache
	refreshTTL  time.Duration
	refreshChan chan struct{}
}

func (c *cachingClient) PerformanceQuery(ctx context.Context, spec vt.PerfQuerySpec, names []string, objs []vt.ManagedObjectReference) (*perfSampleResult, error) {
	return c.delegate.PerformanceQuery(ctx, spec, names, objs)
}

func (c *cachingClient) EnsureConnection(ctx context.Context, logger *zap.Logger) error {
	if vc, ok := c.delegate.(*vcenterClient); ok {
		if vc.moClient != nil {
			if sessionActive, _ := vc.moClient.SessionManager.SessionIsActive(ctx); sessionActive {
				return nil
			}
		}
	}
	if c.refreshChan != nil {
		close(c.refreshChan)
	}
	c.refreshChan = make(chan struct{})
	if err := c.delegate.EnsureConnection(ctx, logger); err != nil {
		return err
	}
	if err := c.cache.Purge(); err != nil {
		return err
	}
	if err := c.refresh(ctx); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(c.refreshTTL)
		for {
			select {
			case <-c.refreshChan:
				return
			case <-ticker.C:
				err := c.refresh(ctx)
				if err != nil {
					logger.Error("Error refreshing vCenter data: %v")
				}
			}
		}
	}()
	return nil
}

func (c *cachingClient) refresh(ctx context.Context) error {
	// cache data centers
	dcs, err := c.delegate.Datacenters(ctx)
	if err != nil {
		return err
	}
	if err = c.cache.Set("dcs", dcs); err != nil {
		return err
	}
	// cache resource pools
	rps, err := c.delegate.ResourcePools(ctx)
	if err != nil {
		return err
	}
	if err = c.cache.Set("resourcepools", rps); err != nil {
		return err
	}
	// cache VMs
	vms, err := c.delegate.VMs(ctx)
	if err != nil {
		return err
	}
	if err = c.cache.Set("vms", vms); err != nil {
		return err
	}

	// cache clusters under data centers
	for _, datacenter := range dcs {
		key := "datacenter" + datacenter.Reference().String()

		clusters, err := c.delegate.Clusters(ctx, datacenter)
		if err != nil {
			return err
		}
		if err = c.cache.Set(key, clusters); err != nil {
			return err
		}
	}

	return nil
}

func (c *cachingClient) Datacenters(_ context.Context) ([]*object.Datacenter, error) {
	dcs, _ := c.cache.Get("dcs")
	return dcs.([]*object.Datacenter), nil
}

func (c *cachingClient) Clusters(_ context.Context, datacenter *object.Datacenter) ([]*object.ClusterComputeResource, error) {
	key := "datacenter" + datacenter.Reference().String()
	clusters, _ := c.cache.Get(key)
	return clusters.([]*object.ClusterComputeResource), nil
}

func (c *cachingClient) ResourcePools(_ context.Context) ([]*object.ResourcePool, error) {
	rps, _ := c.cache.Get("resourcepools")
	return rps.([]*object.ResourcePool), nil
}

func (c *cachingClient) VMs(_ context.Context) ([]*object.VirtualMachine, error) {
	vms, _ := c.cache.Get("vms")
	return vms.([]*object.VirtualMachine), nil
}

func (c *cachingClient) Disconnect(ctx context.Context) error {
	if c.refreshChan != nil {
		close(c.refreshChan)
	}
	_ = c.cache.Close()
	return c.delegate.Disconnect(ctx)
}

var _ client = &cachingClient{}

// vcenterClient is a client that collects data from a vCenter endpoint.
type vcenterClient struct {
	moClient  *govmomi.Client
	vimDriver *vim25.Client
	finder    *find.Finder
	pc        *property.Collector
	pm        *performance.Manager
	cfg       *Config
}

var newVcenterClient = newCachingClient

func defaultNewVcenterClient(c *Config) client {
	return &vcenterClient{
		cfg: c,
	}
}

// EnsureConnection will establish a connection to the vSphere SDK if not already established
func (vc *vcenterClient) EnsureConnection(ctx context.Context, _ *zap.Logger) error {
	if vc.moClient != nil {
		sessionActive, _ := vc.moClient.SessionManager.SessionIsActive(ctx)
		if sessionActive {
			return nil
		}
	}

	sdkURL, err := vc.cfg.SDKUrl()
	if err != nil {
		return err
	}
	client, err := govmomi.NewClient(ctx, sdkURL, vc.cfg.Insecure)
	if err != nil {
		return fmt.Errorf("unable to connect to vSphere SDK on listed endpoint: %w", err)
	}
	tlsCfg, err := vc.cfg.LoadTLSConfig()
	if err != nil {
		return err
	}
	if tlsCfg != nil {
		client.DefaultTransport().TLSClientConfig = tlsCfg
	}
	user := url.UserPassword(vc.cfg.Username, string(vc.cfg.Password))
	err = client.Login(ctx, user)
	if err != nil {
		return fmt.Errorf("unable to login to vcenter sdk: %w", err)
	}
	vc.moClient = client
	vc.vimDriver = client.Client
	vc.pc = property.DefaultCollector(vc.vimDriver)
	vc.finder = find.NewFinder(vc.vimDriver)
	vc.pm = performance.NewManager(vc.vimDriver)
	return nil
}

// Disconnect will logout of the autenticated session
func (vc *vcenterClient) Disconnect(ctx context.Context) error {
	if vc.moClient != nil {
		return vc.moClient.Logout(ctx)
	}
	return nil
}

// Datacenters returns the datacenterComputeResources of the vSphere SDK
func (vc *vcenterClient) Datacenters(ctx context.Context) ([]*object.Datacenter, error) {
	datacenters, err := vc.finder.DatacenterList(ctx, "*")
	if err != nil {
		return []*object.Datacenter{}, fmt.Errorf("unable to get datacenter lists: %w", err)
	}
	return datacenters, nil
}

// Clusters returns the clusterComputeResources of the vSphere SDK
func (vc *vcenterClient) Clusters(ctx context.Context, datacenter *object.Datacenter) ([]*object.ClusterComputeResource, error) {
	vc.finder = vc.finder.SetDatacenter(datacenter)
	clusters, err := vc.finder.ClusterComputeResourceList(ctx, "*")
	if err != nil {
		return []*object.ClusterComputeResource{}, fmt.Errorf("unable to get cluster lists: %w", err)
	}
	return clusters, nil
}

// ResourcePools returns the resourcePools in the vSphere SDK
func (vc *vcenterClient) ResourcePools(ctx context.Context) ([]*object.ResourcePool, error) {
	rps, err := vc.finder.ResourcePoolList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve resource pools: %w", err)
	}
	return rps, err
}

func (vc *vcenterClient) VMs(ctx context.Context) ([]*object.VirtualMachine, error) {
	vms, err := vc.finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve vms: %w", err)
	}
	return vms, err
}

type perfSampleResult struct {
	counters map[string]*vt.PerfCounterInfo
	results  []performance.EntityMetric
}

func (vc *vcenterClient) PerformanceQuery(
	ctx context.Context,
	spec vt.PerfQuerySpec,
	names []string,
	objs []vt.ManagedObjectReference,
) (*perfSampleResult, error) {
	if vc.pm == nil {
		return &perfSampleResult{}, nil
	}
	vc.pm.Sort = true
	sample, err := vc.pm.SampleByName(ctx, spec, names, objs)
	if err != nil {
		return nil, err
	}
	result, err := vc.pm.ToMetricSeries(ctx, sample)
	if err != nil {
		return nil, err
	}
	counterInfoByName, err := vc.pm.CounterInfoByName(ctx)
	if err != nil {
		return nil, err
	}
	return &perfSampleResult{
		counters: counterInfoByName,
		results:  result,
	}, nil
}
