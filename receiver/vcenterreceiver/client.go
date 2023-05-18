// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	vt "github.com/vmware/govmomi/vim25/types"
)

// vcenterClient is a client that
type vcenterClient struct {
	moClient  *govmomi.Client
	vimDriver *vim25.Client
	finder    *find.Finder
	pc        *property.Collector
	pm        *performance.Manager
	cfg       *Config
}

var newVcenterClient = defaultNewVcenterClient

func defaultNewVcenterClient(c *Config) *vcenterClient {
	return &vcenterClient{
		cfg: c,
	}
}

// EnsureConnection will establish a connection to the vSphere SDK if not already established
func (vc *vcenterClient) EnsureConnection(ctx context.Context) error {
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
	user := url.UserPassword(vc.cfg.Username, vc.cfg.Password)
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

// Clusters returns the clusterComputeResources of the vSphere SDK
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

func (vc *vcenterClient) performanceQuery(
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
