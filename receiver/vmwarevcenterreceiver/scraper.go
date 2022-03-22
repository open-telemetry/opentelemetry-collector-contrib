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

	for _, c := range clusters {
		rm := rms.AppendEmpty()
		v.collectCluster(ctx, c, rm, errs)
		v.collectHosts(ctx, c, rms, errs)
	}

	return nil
}

func (v *vmwareVcenterScraper) collectCluster(
	ctx context.Context,
	c mo.ClusterComputeResource,
	rm pdata.ResourceMetrics,
	errs *scrapererror.ScrapeErrors,
) {
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.Cluster, c.Name)

	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	v.collectClusterVSAN(ctx, c, errs)
	v.mb.EmitCluster(ilms.Metrics())
}

func (v *vmwareVcenterScraper) collectClusterVSAN(
	ctx context.Context,
	cluster mo.ClusterComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	mor := cluster.Reference()
	v.logger.Info(fmt.Sprintf("cluster: %s", mor.Value))
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
			v.logger.Info(fmt.Sprintf("metricId: %s", val.MetricId.Label))
			values := strings.Split(val.Values, ",")
			for _, value := range values {
				v.recordClusterVsanMetric(ts, val.MetricId.Label, cluster.Name, value, errs)
			}
		}
	}
}

func (v *vmwareVcenterScraper) collectHosts(
	ctx context.Context,
	cluster mo.ClusterComputeResource,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	hosts, err := v.client.Hosts(ctx)
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

		v.collectHostVsan(ctx, cluster, h, errs)
		v.mb.EmitHost(ilms.Metrics())
	}

}

func (v *vmwareVcenterScraper) collectHostVsan(
	ctx context.Context,
	cluster mo.ClusterComputeResource,
	host *object.HostSystem,
	errs *scrapererror.ScrapeErrors,
) {
	mor := cluster.Reference()
	csvs, err := v.client.CollectVSANHost(ctx, &mor, time.Now(), time.Now())
	if err != nil {
		v.logger.Error(fmt.Sprintf("got an error hosts vsan: %s", err.Error()))
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
			v.logger.Info(fmt.Sprintf("metricId: %s", val.MetricId.Label))
			values := strings.Split(val.Values, ",")
			for _, value := range values {
				v.recordHostVsanMetric(ts, val.MetricId.Label, host.Name(), value, errs)
			}
		}
	}
}

// func (v *vmwareVcenterScraper) collectVMs() {

// }
