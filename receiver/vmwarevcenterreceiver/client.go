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

package vmwarevcenterreceiver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	vt "github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vsan"
	"github.com/vmware/govmomi/vsan/types"
	"go.uber.org/zap"
)

// VmwareVcenterClient is a client that
type VmwareVcenterClient struct {
	moClient   *govmomi.Client
	vimDriver  *vim25.Client
	vsanDriver *vsan.Client
	finder     *find.Finder
	pc         *property.Collector
	cfg        *Config
	logger     *zap.Logger
}

func newVmwarevcenterClient(c *Config, debuglogger *zap.Logger) *VmwareVcenterClient {
	return &VmwareVcenterClient{
		cfg:    c,
		logger: debuglogger,
	}
}

func (vc *VmwareVcenterClient) Connect(ctx context.Context) error {
	if vc.moClient == nil {
		sdkURL, err := vc.cfg.SDKUrl()
		if err != nil {
			return err
		}
		client, err := govmomi.NewClient(ctx, sdkURL, vc.cfg.Insecure)
		if err != nil {
			return err
		}
		tlsCfg, err := vc.cfg.LoadTLSConfig()
		if err != nil {
			return err
		}
		client.DefaultTransport().TLSClientConfig = tlsCfg
		user := url.UserPassword(vc.cfg.Username, vc.cfg.Password)
		err = client.Login(ctx, user)
		if err != nil {
			return fmt.Errorf("unable to login to vcenter sdk: %w", err)
		}
		vc.moClient = client
		vc.vimDriver = client.Client
		vc.pc = property.DefaultCollector(vc.vimDriver)
		vc.finder = find.NewFinder(vc.vimDriver)
	}
	return nil
}

func (vc *VmwareVcenterClient) Disconnect(ctx context.Context) error {
	if vc.moClient != nil {
		return vc.moClient.Logout(ctx)
	}
	return nil
}

func (vc *VmwareVcenterClient) ConnectVSAN(ctx context.Context) error {
	vsanDriver, err := vsan.NewClient(ctx, vc.vimDriver)
	if err != nil {
		return err
	}
	vc.vsanDriver = vsanDriver
	return nil
}

func (vc *VmwareVcenterClient) Clusters(ctx context.Context) ([]*object.ClusterComputeResource, error) {
	clusters, err := vc.finder.ClusterComputeResourceList(ctx, "*")
	if err != nil {
		return []*object.ClusterComputeResource{}, err
	}
	return clusters, nil
}

func (vc *VmwareVcenterClient) Hosts(ctx context.Context) ([]*object.HostSystem, error) {
	hss, err := vc.finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve host list %w", err)
	}
	return hss, nil
}

func (vc *VmwareVcenterClient) DataStores(ctx context.Context) ([]*object.Datastore, error) {
	dss, err := vc.finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve datastores %w", err)
	}
	return dss, err
}

func (vc *VmwareVcenterClient) ResourcePools(ctx context.Context) ([]*object.ResourcePool, error) {
	rps, err := vc.finder.ResourcePoolList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve resource pools: %w", err)
	}
	return rps, err
}

func (vc *VmwareVcenterClient) VMs(ctx context.Context) ([]*object.VirtualMachine, error) {
	rps, err := vc.finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve resource pools: %w", err)
	}
	return rps, err
}

func (vc *VmwareVcenterClient) CollectVSANCluster(ctx context.Context, clusterRef *vt.ManagedObjectReference, startTime time.Time, endTime time.Time) (*[]types.VsanPerfEntityMetricCSV, error) {
	if vc.vsanDriver == nil {
		return nil, errors.New("vsan client not instantiated")
	}
	querySpec := []types.VsanPerfQuerySpec{
		{
			EntityRefId: "cluster-domclient:*",
			StartTime:   &startTime,
			EndTime:     &endTime,
		},
	}
	return vc.queryVsan(ctx, clusterRef, querySpec)
}

func (vc *VmwareVcenterClient) RetrieveProperty(
	ctx context.Context,
	ref vt.ManagedObjectReference,
	path []string,
	dst interface{},
) error {
	return vc.pc.RetrieveOne(ctx, ref, path, dst)
}

func (vc *VmwareVcenterClient) CollectVSANHosts(ctx context.Context, clusterRef *vt.ManagedObjectReference, startTime time.Time, endTime time.Time) (*[]types.VsanPerfEntityMetricCSV, error) {
	if vc.vsanDriver == nil {
		return nil, errors.New("vsan client not instantiated")
	}
	querySpec := []types.VsanPerfQuerySpec{
		{
			EntityRefId: "host-domclient:*",
			StartTime:   &startTime,
			EndTime:     &endTime,
		},
	}
	return vc.queryVsan(ctx, clusterRef, querySpec)
}

func (vc *VmwareVcenterClient) CollectVSANVirtualMachine(
	ctx context.Context,
	clusterRef *vt.ManagedObjectReference,
	startTime time.Time,
	endTime time.Time,
) (*[]types.VsanPerfEntityMetricCSV, error) {
	if vc.vsanDriver == nil {
		return nil, errors.New("vsan client not instantiated")
	}

	querySpec := []types.VsanPerfQuerySpec{
		{
			EntityRefId: "virtual-machine:*",
			StartTime:   &startTime,
			EndTime:     &endTime,
		},
	}
	return vc.queryVsan(ctx, clusterRef, querySpec)
}

func (vc *VmwareVcenterClient) queryVsan(ctx context.Context, ref *vt.ManagedObjectReference, qs []types.VsanPerfQuerySpec) (*[]types.VsanPerfEntityMetricCSV, error) {
	CSVs, err := vc.vsanDriver.VsanPerfQueryPerf(ctx, ref, qs)
	if err != nil {
		return nil, err
	}
	return &CSVs, nil
}
