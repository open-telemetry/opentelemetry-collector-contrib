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

// Datacenters returns the Datacenters of the vSphere SDK
func (vc *vcenterClient) Datacenters(ctx context.Context) ([]mo.Datacenter, error) {
	v, err := vc.vm.CreateContainerView(ctx, vc.vimDriver.ServiceContent.RootFolder, []string{"Datacenter"}, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Datacenters: %w", err)
	}

	var datacenters []mo.Datacenter
	err = v.Retrieve(ctx, []string{"Datacenter"}, []string{
		"name",
	}, &datacenters)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Datacenters: %w", err)
	}

	return datacenters, nil
}

// Datastores returns the Datastores of the vSphere SDK
func (vc *vcenterClient) Datastores(ctx context.Context, containerMoRef vt.ManagedObjectReference) ([]mo.Datastore, error) {
	v, err := vc.vm.CreateContainerView(ctx, containerMoRef, []string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Datastores: %w", err)
	}

	var datastores []mo.Datastore
	err = v.Retrieve(ctx, []string{"Datastore"}, []string{
		"name",
		"summary.capacity",
		"summary.freeSpace",
	}, &datastores)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Datastores: %w", err)
	}

	return datastores, nil
}

// ComputeResources returns the ComputeResources (& ClusterComputeResources) of the vSphere SDK
func (vc *vcenterClient) ComputeResources(ctx context.Context, containerMoRef vt.ManagedObjectReference) ([]mo.ComputeResource, error) {
	v, err := vc.vm.CreateContainerView(ctx, containerMoRef, []string{"ComputeResource"}, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve ComputeResources (& ClusterComputeResources): %w", err)
	}

	var computes []mo.ComputeResource
	err = v.Retrieve(ctx, []string{"ComputeResource"}, []string{
		"name",
		"datastore",
		"host",
		"summary",
	}, &computes)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve ComputeResources (& ClusterComputeResources): %w", err)
	}

	return computes, nil
}

// HostSystems returns the HostSystems of the vSphere SDK
func (vc *vcenterClient) HostSystems(ctx context.Context, containerMoRef vt.ManagedObjectReference) ([]mo.HostSystem, error) {
	v, err := vc.vm.CreateContainerView(ctx, containerMoRef, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve HostSystems: %w", err)
	}

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{
		"name",
		"summary.hardware.memorySize",
		"summary.hardware.numCpuCores",
		"summary.hardware.cpuMhz",
		"summary.quickStats.overallMemoryUsage",
		"summary.quickStats.overallCpuUsage",
		"vm",
		"parent",
	}, &hosts)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve HostSystems: %w", err)
	}

	return hosts, nil
}

// ResourcePools returns the ResourcePools (&VirtualApps) of the vSphere SDK
func (vc *vcenterClient) ResourcePools(ctx context.Context, containerMoRef vt.ManagedObjectReference) ([]mo.ResourcePool, error) {
	v, err := vc.vm.CreateContainerView(ctx, containerMoRef, []string{"ResourcePool"}, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve ResourcePools (&VirtualApps): %w", err)
	}

	var rps []mo.ResourcePool
	err = v.Retrieve(ctx, []string{"ResourcePool"}, []string{
		"summary",
		"name",
		"owner",
		"vm",
	}, &rps)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve ResourcePools (&VirtualApps): %w", err)
	}

	return rps, nil
}

// VMS returns the VirtualMachines of the vSphere SDK
func (vc *vcenterClient) VMs(ctx context.Context, containerMoRef vt.ManagedObjectReference) ([]mo.VirtualMachine, error) {
	v, err := vc.vm.CreateContainerView(ctx, containerMoRef, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve VMs: %w", err)
	}

	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{
		"name",
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

// DatacenterInventoryListObjects returns the Datacenters (with populated InventoryLists) of the vSphere SDK
func (vc *vcenterClient) DatacenterInventoryListObjects(ctx context.Context) ([]*object.Datacenter, error) {
	dcs, err := vc.finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Datacenters with InventoryLists: %w", err)
	}

	return dcs, nil
}

// ResourcePoolInventoryListObjects returns the ResourcePools (with populated InventoryLists) of the vSphere SDK
func (vc *vcenterClient) ResourcePoolInventoryListObjects(
	ctx context.Context,
	dcs []*object.Datacenter,
) ([]*object.ResourcePool, error) {
	allRPools := []*object.ResourcePool{}
	for _, dc := range dcs {
		vc.finder.SetDatacenter(dc)
		rps, err := vc.finder.ResourcePoolList(ctx, "*")
		var notFoundErr *find.NotFoundError
		if err != nil && !errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("unable to retrieve ResourcePools with InventoryLists for datacenter %s: %w", dc.InventoryPath, err)
		}
		allRPools = append(allRPools, rps...)
	}

	return allRPools, nil
}

// VAppInventoryListObjects returns the vApps (with populated InventoryLists) of the vSphere SDK
func (vc *vcenterClient) VAppInventoryListObjects(
	ctx context.Context,
	dcs []*object.Datacenter,
) ([]*object.VirtualApp, error) {
	allVApps := []*object.VirtualApp{}
	for _, dc := range dcs {
		vc.finder.SetDatacenter(dc)
		vApps, err := vc.finder.VirtualAppList(ctx, "*")
		if err == nil {
			allVApps = append(allVApps, vApps...)
			continue
		}

		var notFoundErr *find.NotFoundError
		if !errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("unable to retrieve vApps with InventoryLists for datacenter %s: %w", dc.InventoryPath, err)
		}
	}

	return allVApps, nil
}

// PerfMetricsQueryResult contains performance metric related data
type PerfMetricsQueryResult struct {
	// Contains performance metrics keyed by MoRef string
	resultsByRef map[string]*performance.EntityMetric
}

// PerfMetricsQuery returns the requested performance metrics for the requested resources
// over a given sample interval and sample count
func (vc *vcenterClient) PerfMetricsQuery(
	ctx context.Context,
	spec vt.PerfQuerySpec,
	names []string,
	objs []vt.ManagedObjectReference,
) (*PerfMetricsQueryResult, error) {
	if vc.pm == nil {
		return &PerfMetricsQueryResult{}, nil
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

	resultsByRef := map[string]*performance.EntityMetric{}
	for i := range result {
		resultsByRef[result[i].Entity.Value] = &result[i]
	}
	return &PerfMetricsQueryResult{
		resultsByRef: resultsByRef,
	}, nil
}
