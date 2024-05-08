// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	vt "github.com/vmware/govmomi/vim25/types"
)

// vcenterClient is a client that collects data from a vCenter endpoint.
type vcenterClient struct {
	moClient  *govmomi.Client
	vimDriver *vim25.Client
	finder    *find.Finder
	pc        *property.Collector
	pm        *performance.Manager
	vm        *view.Manager
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
	tlsCfg, err := vc.cfg.LoadTLSConfig(ctx)
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
	vc.vm = view.NewManager(vc.vimDriver)
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

// Computes returns the ComputeResources (and ClusterComputeResources) of the vSphere SDK for a given datacenter
func (vc *vcenterClient) Computes(ctx context.Context, datacenter *object.Datacenter) ([]*object.ComputeResource, error) {
	vc.finder = vc.finder.SetDatacenter(datacenter)
	computes, err := vc.finder.ComputeResourceList(ctx, "*")
	if err != nil {
		return []*object.ComputeResource{}, fmt.Errorf("unable to get compute lists: %w", err)
	}
	return computes, nil
}

// ResourcePools returns the ResourcePools in the vSphere SDK
func (vc *vcenterClient) ResourcePools(ctx context.Context) ([]*object.ResourcePool, error) {
	rps, err := vc.finder.ResourcePoolList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve resource pools: %w", err)
	}
	return rps, err
}

// VirtualApps returns the VirtualApps in the vSphere SDK
func (vc *vcenterClient) VirtualApps(ctx context.Context) ([]*object.VirtualApp, error) {
	vApps, err := vc.finder.VirtualAppList(ctx, "*")
	if err != nil {
		var notFoundErr *find.NotFoundError
		if errors.As(err, &notFoundErr) {
			return []*object.VirtualApp{}, nil
		}

		return nil, fmt.Errorf("unable to retrieve vApps: %w", err)
	}
	return vApps, err
}

func (vc *vcenterClient) VMs(ctx context.Context) ([]mo.VirtualMachine, error) {
	v, err := vc.vm.CreateContainerView(ctx, vc.vimDriver.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve VMs: %w", err)
	}

	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{
		"config.hardware.numCPU",
		"config.instanceUuid",
		"config.template",
		"runtime.powerState",
		"runtime.maxCpuUsage",
		"summary.quickStats.guestMemoryUsage",
		"summary.quickStats.balloonedMemory",
		"summary.quickStats.swappedMemory",
		"summary.quickStats.ssdSwappedMemory",
		"summary.quickStats.overallCpuUsage",
		"summary.config.memorySizeMB",
		"summary.config.name",
		"summary.storage.committed",
		"summary.storage.uncommitted",
		"summary.runtime.host",
		"resourcePool",
		"parentVApp",
	}, &vms)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve VMs: %w", err)
	}

	return vms, nil
}

type perfSampleResult struct {
	counters map[string]*vt.PerfCounterInfo
	results  []performance.EntityMetric
}

type perfMetricsQueryResult struct {
	counters       map[string]*vt.PerfCounterInfo
	resultsByMoRef map[string]*performance.EntityMetric
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

func (vc *vcenterClient) perfMetricsQuery(
	ctx context.Context,
	spec vt.PerfQuerySpec,
	names []string,
	objs []vt.ManagedObjectReference,
) (*perfMetricsQueryResult, error) {
	if vc.pm == nil {
		return &perfMetricsQueryResult{}, nil
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

	resultsByMoRef := map[string]*performance.EntityMetric{}
	for i := range result {
		resultsByMoRef[result[i].Entity.Value] = &result[i]
	}
	return &perfMetricsQueryResult{
		counters:       counterInfoByName,
		resultsByMoRef: resultsByMoRef,
	}, nil
}
