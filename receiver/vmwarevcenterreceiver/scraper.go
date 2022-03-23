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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vsan/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver/internal/metadata"
)

// example 2022-03-10 14:15:00
const timeFormat = "2006-01-02 15:04:05"

const instrumentationLibraryName = "otelcol/vcenter"

type vmwareVcenterScraper struct {
	client *VmwareVcenterClient
	logger *zap.Logger
	config *Config
	mb     *metadata.MetricsBuilder
}

func newVmwareVcenterScraper(
	logger *zap.Logger,
	config *Config,
) *vmwareVcenterScraper {
	l := logger.Named("vcenter-client")
	client := newVmwarevcenterClient(config, l)
	return &vmwareVcenterScraper{
		logger: logger,
		client: client,
		config: config,
		mb:     metadata.NewMetricsBuilder(config.Metrics),
	}
}

func (v *vmwareVcenterScraper) start(ctx context.Context, host component.Host) error {
	return v.client.Connect(ctx)
}

func (v *vmwareVcenterScraper) shutdown(ctx context.Context) error {
	return v.client.Disconnect(ctx)
}

func (v *vmwareVcenterScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	if v.client == nil {
		return pdata.Metrics{}, errors.New("failed to connect to http client")
	}

	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()
	errs := &scrapererror.ScrapeErrors{}

	err := v.client.ConnectVSAN(ctx)
	if err != nil {
		errs.AddPartial(1, err)
	}

	v.collectClusters(ctx, rms, errs)

	return metrics, errs.Combine()
}

func (v *vmwareVcenterScraper) collectClusters(ctx context.Context, rms pdata.ResourceMetricsSlice, errs *scrapererror.ScrapeErrors) error {
	clusters, err := v.client.Clusters(ctx)
	if err != nil {
		return err
	}
	now := pdata.NewTimestampFromTime(time.Now())

	for _, c := range clusters {
		rm := rms.AppendEmpty()
		v.collectCluster(ctx, c, rm, errs)
		v.collectHosts(ctx, now, c, rms, errs)
		v.collectDatastores(ctx, now, c, rms, errs)
		v.collectVMs(ctx, now, c, rms, errs)
		v.collectResourcePools(ctx, rms, errs)
	}

	return nil
}

func (v *vmwareVcenterScraper) collectCluster(
	ctx context.Context,
	c *object.ClusterComputeResource,
	rm pdata.ResourceMetrics,
	errs *scrapererror.ScrapeErrors,
) {
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.Cluster, c.Name())

	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	v.collectClusterVSAN(ctx, c, errs)
	v.mb.EmitCluster(ilms.Metrics())
}

func (v *vmwareVcenterScraper) collectClusterVSAN(
	ctx context.Context,
	cluster *object.ClusterComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	mor := cluster.Reference()
	csvs, err := v.client.CollectVSANCluster(ctx, &mor, time.Now(), time.Now())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, r := range *csvs {
		time, err := time.Parse(timeFormat, r.SampleInfo)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}
		ts := pdata.NewTimestampFromTime(time)

		for _, val := range r.Value {
			values := strings.Split(val.Values, ",")
			for _, value := range values {
				v.recordClusterVsanMetric(ts, val.MetricId.Label, cluster.Name(), value, errs)
			}
		}
	}
}

func (v *vmwareVcenterScraper) collectDatastores(
	ctx context.Context,
	colTime pdata.Timestamp,
	_ *object.ClusterComputeResource,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	datastores, err := v.client.DataStores(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for _, ds := range datastores {
		v.logger.Info(fmt.Sprintf("datastore: %s", ds.Name()))
		v.collectDatastore(ctx, colTime, rms, ds)
	}
}

func (v *vmwareVcenterScraper) collectDatastore(
	_ context.Context,
	_ pdata.Timestamp,
	rms pdata.ResourceMetricsSlice,
	ds *object.Datastore,
) {
	rm := rms.AppendEmpty()
	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.Datastore, ds.Name())

	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	v.mb.EmitDatastore(ilms.Metrics())
}

func (v *vmwareVcenterScraper) collectHosts(
	ctx context.Context,
	colTime pdata.Timestamp,
	cluster *object.ClusterComputeResource,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	hosts, err := v.client.Hosts(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	clusterRef := cluster.Reference()
	hostVsanCSVs, err := v.client.CollectVSANHosts(ctx, &clusterRef, time.Now(), time.Now())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, h := range hosts {
		rm := rms.AppendEmpty()
		resourceAttrs := rm.Resource().Attributes()
		resourceAttrs.InsertString("hostname", h.Name())

		ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

		v.collectHost(ctx, colTime, h, hostVsanCSVs, errs)
		v.mb.EmitHost(ilms.Metrics())
	}

}

func (v *vmwareVcenterScraper) collectHost(
	ctx context.Context,
	now pdata.Timestamp,
	host *object.HostSystem,
	vsanCsvs *[]types.VsanPerfEntityMetricCSV,
	errs *scrapererror.ScrapeErrors,
) {
	var hwSum mo.HostSystem
	err := host.Properties(ctx, host.Reference(), []string{"summary.hardware"}, &hwSum)
	if err != nil {
		v.logger.Error(err.Error())
		return
	}
	v.recordHostSystemMemoryUsage(now, hwSum, host.Name(), errs)

	if vsanCsvs != nil {
		entityRef := fmt.Sprintf("host-domclient:%v",
			hwSum.Summary.Hardware.Uuid,
		)
		v.addVSAN(*vsanCsvs, entityRef, host.Name(), hostType, errs)
	}
}

func (v *vmwareVcenterScraper) collectResourcePools(
	ctx context.Context,
	_ pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	rps, err := v.client.ResourcePools(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for _, rp := range rps {
		v.logger.Info(fmt.Sprintf("collecting resource pool: %s", rp.Name()))
		// v.collectDatastore(ctx, rms, rp)
	}
}

func (v *vmwareVcenterScraper) collectVMs(
	ctx context.Context,
	_ pdata.Timestamp,
	cluster *object.ClusterComputeResource,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	vms, err := v.client.VMs(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	clusterRef := cluster.Reference()
	vsanCsvs, err := v.client.CollectVSANVirtualMachine(ctx, &clusterRef, time.Now(), time.Now())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, vm := range vms {
		rm := rms.AppendEmpty()
		resourceAttrs := rm.Resource().Attributes()
		resourceAttrs.InsertString(metadata.A.InstanceName, vm.Name())

		ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
		v.logger.Info(fmt.Sprintf("virtual machine: %s %s", vm.Name(), vm.UUID(ctx)))
		v.collectVM(ctx, vm, vsanCsvs, errs)
		v.mb.EmitVM(ilms.Metrics())
	}

}

func (v *vmwareVcenterScraper) collectVM(
	ctx context.Context,
	vm *object.VirtualMachine,
	vsanCsvs *[]types.VsanPerfEntityMetricCSV,
	errs *scrapererror.ScrapeErrors,
) {
	if vsanCsvs != nil {
		vmUUID := vm.UUID(ctx)
		entityRef := fmt.Sprintf("virtual-machine: %s %s", vm.Name(), vmUUID)
		v.addVSAN(*vsanCsvs, entityRef, vm.Name(), vmType, errs)
	}
}

type vsanType int

const (
	clusterType vsanType = iota
	hostType
	vmType
)

func (v *vmwareVcenterScraper) addVSAN(
	csvs []types.VsanPerfEntityMetricCSV,
	entityID string,
	entityName string,
	vsanType vsanType,
	errs *scrapererror.ScrapeErrors,
) {
	for _, r := range csvs {
		v.logger.Info(r.EntityRefId)
		if r.EntityRefId != entityID {
			continue
		}

		time, err := time.Parse(timeFormat, r.SampleInfo)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}

		ts := pdata.NewTimestampFromTime(time)
		for _, val := range r.Value {
			values := strings.Split(val.Values, ",")
			for _, value := range values {
				switch vsanType {
				case clusterType:
					v.recordClusterVsanMetric(ts, val.MetricId.Label, entityName, value, errs)
				case hostType:
					v.recordHostVsanMetric(ts, val.MetricId.Label, entityName, value, errs)
				case vmType:
					v.recordVMVsanMetric(ts, val.MetricId.Label, entityName, value, errs)
				}
			}
		}
	}
}
