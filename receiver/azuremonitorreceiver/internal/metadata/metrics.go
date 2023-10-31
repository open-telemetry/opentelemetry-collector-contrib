// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
)

const metricsPrefix = "azure_"

type SemanticConventionResourceAttribute string

func NewMetricsBuilder(mbc MetricsBuilderConfig, settings receiver.CreateSettings, options ...metricBuilderOption) *MetricsBuilder {
	mb := &MetricsBuilder{
		config:        mbc,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		metricsBuffer: pmetric.NewMetrics(),
		buildInfo:     settings.BuildInfo,
		metrics:       map[string]*metricAzureAbstract{},
	}
	for _, op := range options {
		op(mb)
	}
	return mb
}

func DefaultMetricsBuilderConfig() MetricsBuilderConfig {
	return MetricsBuilderConfig{
		ResourceAttributes: DefaultResourceAttributesConfig(),
	}
}

// MetricsBuilderConfig is a structural subset of an otherwise 1-1 copy of metadata.yaml
type MetricsBuilderConfig struct {
	ResourceAttributes ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

// MetricsBuilder provides an interface for scrapers to report metrics while taking care of all the transformations
// required to produce metric representation defined in metadata and user settings.
type MetricsBuilder struct {
	config           MetricsBuilderConfig
	startTime        pcommon.Timestamp   // start time that will be applied to all recorded data points.
	metricsCapacity  int                 // maximum observed number of metrics per resource.
	resourceCapacity int                 // maximum observed number of resource attributes.
	metricsBuffer    pmetric.Metrics     // accumulates metrics data before emitting.
	buildInfo        component.BuildInfo // contains version information
	metrics          map[string]*metricAzureAbstract
}

// metricBuilderOption applies changes to default metrics builder.
type metricBuilderOption func(*MetricsBuilder)

// updateCapacity updates max length of metrics and resource attributes that will be used for the slice capacity.
func (mb *MetricsBuilder) updateCapacity(rm pmetric.ResourceMetrics) {
	if mb.metricsCapacity < rm.ScopeMetrics().At(0).Metrics().Len() {
		mb.metricsCapacity = rm.ScopeMetrics().At(0).Metrics().Len()
	}
	if mb.resourceCapacity < rm.Resource().Attributes().Len() {
		mb.resourceCapacity = rm.Resource().Attributes().Len()
	}
}

// ResourceMetricsOption applies changes to provided resource metrics.
type ResourceMetricsOption func(ResourceAttributesConfig, pmetric.ResourceMetrics)

// WithAzuremonitorSubscriptionID sets provided value as "azuremonitor.subscription_id" attribute for current resource.
func WithAzuremonitorSubscriptionID(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
		if ras.AzuremonitorSubscriptionID.Enabled {
			rm.Resource().Attributes().PutStr("azuremonitor.subscription_id", val)
		}
	}
}

// WithAzuremonitorTenantID sets provided value as "azuremonitor.tenant_id" attribute for current resource.
func WithAzuremonitorTenantID(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
		if ras.AzuremonitorTenantID.Enabled {
			rm.Resource().Attributes().PutStr("azuremonitor.tenant_id", val)
		}
	}
}

// WithCloudProvider sets provided value as "cloud.provider" attribute for current resource.
func WithCloudProvider(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
		if ras.CloudProvider.Enabled {
			rm.Resource().Attributes().PutStr("cloud.provider", val)
		}
	}
}

// WithCloudAccountID sets provided value as "cloud.account.id" attribute for current resource.
func WithCloudAccountID(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
		if ras.CloudAccountID.Enabled {
			rm.Resource().Attributes().PutStr("cloud.account.id", val)
		}
	}
}

// WithCloudRegion sets provided value as "cloud.region" attribute for current resource.
func WithCloudRegion(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
		if ras.CloudRegion.Enabled {
			rm.Resource().Attributes().PutStr("cloud.region", val)
		}
	}
}

// WithCloudAvailabilityZone sets provided value as "cloud.availability_zone" attribute for current resource.
func WithCloudAvailabilityZone(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
		if ras.CloudAvailabilityZone.Enabled {
			rm.Resource().Attributes().PutStr("cloud.availability_zone", val)
		}
	}
}

// WithCloudPlatform sets provided value as "cloud.platform" attribute for current resource.
func WithCloudPlatform(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
		if ras.CloudPlatform.Enabled {
			rm.Resource().Attributes().PutStr("cloud.platform", val)
		}
	}
}

// EmitForResource saves all the generated metrics under a new resource and updates the internal state to be ready for
// recording another set of data points as part of another resource. This function can be helpful when one scraper
// needs to emit metrics from several resources. Otherwise calling this function is not required,
// just `Emit` function can be called instead.
// Resource attributes should be provided as ResourceMetricsOption arguments.
func (mb *MetricsBuilder) EmitForResource(rmo ...ResourceMetricsOption) {
	rm := pmetric.NewResourceMetrics()
	rm.Resource().Attributes().EnsureCapacity(mb.resourceCapacity)
	ils := rm.ScopeMetrics().AppendEmpty()
	ils.Scope().SetName("otelcol/azuremonitorreceiver")
	ils.Scope().SetVersion(mb.buildInfo.Version)
	ils.Metrics().EnsureCapacity(mb.metricsCapacity)
	mb.EmitAllMetrics(ils)

	for _, op := range rmo {
		op(mb.config.ResourceAttributes, rm)
	}
	if ils.Metrics().Len() > 0 {
		mb.updateCapacity(rm)
		rm.MoveTo(mb.metricsBuffer.ResourceMetrics().AppendEmpty())
	}
}

// Emit returns all the metrics accumulated by the metrics builder and updates the internal state to be ready for
// recording another set of metrics. This function will be responsible for applying all the transformations required to
// produce metric representation defined in metadata and user settings, e.g. delta or cumulative.
func (mb *MetricsBuilder) Emit(rmo ...ResourceMetricsOption) pmetric.Metrics {
	mb.EmitForResource(rmo...)
	metrics := mb.metricsBuffer
	mb.metricsBuffer = pmetric.NewMetrics()
	return metrics
}

// Reset resets metrics builder to its initial state. It should be used when external metrics source is restarted,
// and metrics builder should update its startTime and reset it's internal state accordingly.
func (mb *MetricsBuilder) Reset(options ...metricBuilderOption) {
	mb.startTime = pcommon.NewTimestampFromTime(time.Now())
	for _, op := range options {
		op(mb)
	}
}

type metricAzureAbstract struct {
	data     pmetric.Metric // data buffer for generated metric.
	capacity int            // max observed number of data points added to the metric.
}

func (m *metricAzureAbstract) updateCapacity() {
	if m.data.Gauge().DataPoints().Len() > m.capacity {
		m.capacity = m.data.Gauge().DataPoints().Len()
	}
}

func (m *metricAzureAbstract) init(name, unit string) {
	m.data.SetName(name)
	m.data.SetUnit(unit)
	m.data.SetEmptyGauge()
	m.data.Gauge().DataPoints().EnsureCapacity(m.capacity)
}

func (mb *MetricsBuilder) getMetric(resourceMetricID string) (*metricAzureAbstract, bool) {
	if _, exists := mb.metrics[resourceMetricID]; !exists {
		return nil, false
	}
	return mb.metrics[resourceMetricID], true
}

func (mb *MetricsBuilder) addMetric(resourceMetricID, logicalMetricID, unit string) (*metricAzureAbstract, error) {
	if _, exists := mb.metrics[resourceMetricID]; exists {
		return nil, errors.New("metric already exists")
	}

	m := &metricAzureAbstract{}
	m.data = pmetric.NewMetric()

	m.init(logicalMetricID, unit)

	mb.metrics[resourceMetricID] = m

	return mb.metrics[resourceMetricID], nil
}

func (mb *MetricsBuilder) AddDataPoint(
	resourceID,
	metric,
	aggregation,
	unit string,
	attributes map[string]*string,
	ts pcommon.Timestamp,
	val float64,
) {
	logicalMetricID := getLogicalMetricID(metric, aggregation)
	resourceMetricID := getLogicalResourceMetricID(resourceID, logicalMetricID)

	m, exists := mb.getMetric(resourceMetricID)
	if !exists {
		var err error
		m, err = mb.addMetric(resourceMetricID, logicalMetricID, unit)
		if err != nil {
			log.Println(err)
		}
	}
	dp := m.data.Gauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(mb.startTime)
	dp.SetTimestamp(ts)
	dp.SetDoubleValue(val)
	dp.Attributes().PutStr("azuremonitor.resource_id", resourceID)
	for key, value := range attributes {
		dp.Attributes().PutStr(key, *value)
	}
}

func getLogicalMetricID(metric, aggregation string) string {
	return strings.ToLower(fmt.Sprintf("%s%s_%s", metricsPrefix, strings.ReplaceAll(metric, " ", "_"), aggregation))
}

func getLogicalResourceMetricID(resourceID, logicalMetricID string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(resourceID), logicalMetricID)
}

func (mb *MetricsBuilder) EmitAllMetrics(ils pmetric.ScopeMetrics) {
	for _, m := range mb.metrics {
		if m.data.Gauge().DataPoints().Len() > 0 {
			metrics := ils.Metrics()
			m.updateCapacity()
			name := m.data.Name()
			unit := m.data.Unit()
			m.data.MoveTo(metrics.AppendEmpty())
			m.init(name, unit)
		}
	}
}
