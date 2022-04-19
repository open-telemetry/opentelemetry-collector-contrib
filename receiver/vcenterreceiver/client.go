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

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	vt "github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vsan"
	"github.com/vmware/govmomi/vsan/types"
)

// vcenterClient is a client that
type vcenterClient struct {
	moClient   *govmomi.Client
	vimDriver  *vim25.Client
	vsanDriver *vsan.Client
	finder     *find.Finder
	pc         *property.Collector
	cfg        *Config
}

func newVmwarevcenterClient(c *Config) *vcenterClient {
	return &vcenterClient{
		cfg: c,
	}
}

func (vc *vcenterClient) Connect(ctx context.Context) error {
	if vc.moClient == nil {
		sdkURL, err := vc.cfg.SDKUrl()
		if err != nil {
			return err
		}
		client, err := govmomi.NewClient(ctx, sdkURL, vc.cfg.MetricsConfig.Insecure)
		if err != nil {
			return fmt.Errorf("unable to connect to vSphere SDK on listed endpoint: %w", err)
		}
		tlsCfg, err := vc.cfg.MetricsConfig.LoadTLSConfig()
		if err != nil {
			return err
		}
		client.DefaultTransport().TLSClientConfig = tlsCfg
		user := url.UserPassword(vc.cfg.MetricsConfig.Username, vc.cfg.MetricsConfig.Password)
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

func (vc *vcenterClient) Disconnect(ctx context.Context) error {
	if vc.moClient != nil {
		return vc.moClient.Logout(ctx)
	}
	return nil
}

func (vc *vcenterClient) ConnectVSAN(ctx context.Context) error {
	if vc.vsanDriver == nil {
		vsanDriver, err := vsan.NewClient(ctx, vc.vimDriver)
		if err != nil {
			return err
		}
		vc.vsanDriver = vsanDriver
	}
	return nil
}

func (vc *vcenterClient) Clusters(ctx context.Context) ([]*object.ClusterComputeResource, error) {
	clusters, err := vc.finder.ClusterComputeResourceList(ctx, "*")
	if err != nil {
		return []*object.ClusterComputeResource{}, err
	}
	return clusters, nil
}

func (vc *vcenterClient) ResourcePools(ctx context.Context) ([]*object.ResourcePool, error) {
	rps, err := vc.finder.ResourcePoolList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve resource pools: %w", err)
	}
	return rps, err
}

func (vc *vcenterClient) VMs(ctx context.Context) ([]*object.VirtualMachine, error) {
	rps, err := vc.finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve resource pools: %w", err)
	}
	return rps, err
}

func (vc *vcenterClient) VSANCluster(
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
			EntityRefId: "cluster-domclient:*",
			StartTime:   &startTime,
			EndTime:     &endTime,
		},
	}
	return vc.queryVsan(ctx, clusterRef, querySpec)
}

func (vc *vcenterClient) VSANHosts(ctx context.Context, clusterRef *vt.ManagedObjectReference, startTime time.Time, endTime time.Time) (*[]types.VsanPerfEntityMetricCSV, error) {
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

func (vc *vcenterClient) VSANVirtualMachines(
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
	mgr := performance.NewManager(vc.vimDriver)
	mgr.Sort = true
	sample, err := mgr.SampleByName(ctx, spec, names, objs)
	if err != nil {
		return nil, err
	}
	result, err := mgr.ToMetricSeries(ctx, sample)
	if err != nil {
		return nil, err
	}
	counterInfoByName, err := mgr.CounterInfoByName(ctx)
	if err != nil {
		return nil, err
	}
	return &perfSampleResult{
		counters: counterInfoByName,
		results:  result,
	}, nil
}

func (vc *vcenterClient) queryVsan(ctx context.Context, ref *vt.ManagedObjectReference, qs []types.VsanPerfQuerySpec) (*[]types.VsanPerfEntityMetricCSV, error) {
	CSVs, err := vc.vsanDriver.VsanPerfQueryPerf(ctx, ref, qs)
	if err != nil {
		return nil, err
	}
	return &CSVs, nil
}
