// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

var (
	timeGrains = map[string]int64{
		"PT1M":  60,
		"PT5M":  300,
		"PT15M": 900,
		"PT30M": 1800,
		"PT1H":  3600,
		"PT6H":  21600,
		"PT12H": 43200,
		"P1D":   86400,
	}
	aggregations = []string{
		"Average",
		"Count",
		"Maximum",
		"Minimum",
		"Total",
	}
)

const (
	tagPrefix      = "tags_"
	metadataPrefix = "metadata_"
	location       = "location"
)

type azureResource struct {
	metricsByCompositeKey     map[metricsCompositeKey]*azureResourceMetrics
	metricsDefinitionsUpdated time.Time
	tags                      map[string]*string
	location                  string
}

type metricsCompositeKey struct {
	dimensions string // comma separated sorted dimensions
	timeGrain  string
}

type azureResourceMetrics struct {
	metrics              []string
	metricsValuesUpdated time.Time
}

type void struct{}

func newScraper(conf *Config, settings receiver.CreateSettings) *azureScraper {
	return &azureScraper{
		cfg:                             conf,
		settings:                        settings.TelemetrySettings,
		mb:                              metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		azIDCredentialsFunc:             azidentity.NewClientSecretCredential,
		armClientFunc:                   armresources.NewClient,
		armMonitorDefinitionsClientFunc: armmonitor.NewMetricDefinitionsClient,
		armMonitorMetricsClientFunc:     armmonitor.NewMetricsClient,
		mutex:                           &sync.Mutex{},
	}
}

type azureScraper struct {
	cred azcore.TokenCredential

	clientResources          ArmClient
	clientMetricsDefinitions MetricsDefinitionsClientInterface
	clientMetricsValues      MetricsValuesClient

	cfg                             *Config
	settings                        component.TelemetrySettings
	resources                       map[string]*azureResource
	resourcesUpdated                time.Time
	mb                              *metadata.MetricsBuilder
	azIDCredentialsFunc             func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	armClientFunc                   func(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error)
	armMonitorDefinitionsClientFunc func(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error)
	armMonitorMetricsClientFunc     func(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error)
	mutex                           *sync.Mutex
}

type ArmClient interface {
	NewListPager(options *armresources.ClientListOptions) *runtime.Pager[armresources.ClientListResponse]
}

func (s *azureScraper) getArmClient() ArmClient {
	client, _ := s.armClientFunc(s.cfg.SubscriptionID, s.cred, nil)
	return client
}

type MetricsDefinitionsClientInterface interface {
	NewListPager(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) *runtime.Pager[armmonitor.MetricDefinitionsClientListResponse]
}

func (s *azureScraper) getMetricsDefinitionsClient() MetricsDefinitionsClientInterface {
	client, _ := s.armMonitorDefinitionsClientFunc(s.cred, nil)
	return client
}

type MetricsValuesClient interface {
	List(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (
		armmonitor.MetricsClientListResponse, error,
	)
}

func (s *azureScraper) GetMetricsValuesClient() MetricsValuesClient {
	client, _ := s.armMonitorMetricsClientFunc(s.cred, nil)
	return client
}

func (s *azureScraper) start(_ context.Context, _ component.Host) (err error) {
	s.cred, err = s.azIDCredentialsFunc(s.cfg.TenantID, s.cfg.ClientID, s.cfg.ClientSecret, nil)
	if err != nil {
		return err
	}

	s.clientResources = s.getArmClient()
	s.clientMetricsDefinitions = s.getMetricsDefinitionsClient()
	s.clientMetricsValues = s.GetMetricsValuesClient()

	s.resources = map[string]*azureResource{}

	return
}

func (s *azureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {

	s.getResources(ctx)
	resourcesIdsWithDefinitions := make(chan string)

	go func() {
		defer close(resourcesIdsWithDefinitions)
		for resourceID := range s.resources {
			s.getResourceMetricsDefinitions(ctx, resourceID)
			resourcesIdsWithDefinitions <- resourceID
		}
	}()

	var wg sync.WaitGroup
	for resourceID := range resourcesIdsWithDefinitions {
		wg.Add(1)
		go func(resourceID string) {
			defer wg.Done()
			s.getResourceMetricsValues(ctx, resourceID)
		}(resourceID)
	}
	wg.Wait()

	return s.mb.Emit(
		metadata.WithAzureMonitorSubscriptionID(s.cfg.SubscriptionID),
		metadata.WithAzureMonitorTenantID(s.cfg.TenantID),
	), nil
}

func (s *azureScraper) getResources(ctx context.Context) {
	if time.Since(s.resourcesUpdated).Seconds() < s.cfg.CacheResources {
		return
	}
	existingResources := map[string]void{}
	for id := range s.resources {
		existingResources[id] = void{}
	}

	filter := s.getResourcesFilter()
	opts := &armresources.ClientListOptions{
		Filter: &filter,
	}

	pager := s.clientResources.NewListPager(opts)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Resources data", zap.Error(err))
			return
		}
		for _, resource := range nextResult.Value {

			if _, ok := s.resources[*resource.ID]; !ok {
				s.resources[*resource.ID] = &azureResource{tags: resource.Tags}
				if resource.Location != nil {
					s.resources[*resource.ID].location = *resource.Location
				}
			}
			delete(existingResources, *resource.ID)
		}
	}
	if len(existingResources) > 0 {
		for idToDelete := range existingResources {
			delete(s.resources, idToDelete)
		}
	}

	s.resourcesUpdated = time.Now()
}

func (s *azureScraper) getResourcesFilter() string {
	// TODO: switch to parsing services from
	// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported
	resourcesTypeFilter := strings.Join(s.cfg.Services, "' or resourceType eq '")

	resourcesGroupFilterString := ""
	if len(s.cfg.ResourceGroups) > 0 {
		resourcesGroupFilterString = fmt.Sprintf(" and (resourceGroup eq '%s')",
			strings.Join(s.cfg.ResourceGroups, "' or resourceGroup eq  '"))
	}

	return fmt.Sprintf("(resourceType eq '%s')%s", resourcesTypeFilter, resourcesGroupFilterString)
}

func (s *azureScraper) getResourceMetricsDefinitions(ctx context.Context, resourceID string) {

	if time.Since(s.resources[resourceID].metricsDefinitionsUpdated).Seconds() < s.cfg.CacheResourcesDefinitions {
		return
	}

	s.resources[resourceID].metricsByCompositeKey = map[metricsCompositeKey]*azureResourceMetrics{}

	pager := s.clientMetricsDefinitions.NewListPager(resourceID, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Metrics definitions data", zap.Error(err))
			return
		}

		for _, v := range nextResult.Value {

			timeGrain := *v.MetricAvailabilities[0].TimeGrain
			name := *v.Name.Value
			compositeKey := metricsCompositeKey{timeGrain: timeGrain}

			if len(v.Dimensions) > 0 {
				var dimensionsSlice []string
				for _, dimension := range v.Dimensions {
					if len(strings.TrimSpace(*dimension.Value)) > 0 {
						dimensionsSlice = append(dimensionsSlice, *dimension.Value)
					}
				}
				sort.Strings(dimensionsSlice)
				dimensionsCompositeKey := metricsCompositeKey{timeGrain: timeGrain, dimensions: strings.Join(dimensionsSlice, ",")}
				s.storeMetricsDefinition(resourceID, name, dimensionsCompositeKey)
			} else {
				s.storeMetricsDefinition(resourceID, name, compositeKey)
			}
		}
	}
	s.resources[resourceID].metricsDefinitionsUpdated = time.Now()
}

func (s *azureScraper) storeMetricsDefinition(resourceID, name string, compositeKey metricsCompositeKey) {
	if _, ok := s.resources[resourceID].metricsByCompositeKey[compositeKey]; ok {
		s.resources[resourceID].metricsByCompositeKey[compositeKey].metrics = append(
			s.resources[resourceID].metricsByCompositeKey[compositeKey].metrics, name,
		)
	} else {
		s.resources[resourceID].metricsByCompositeKey[compositeKey] = &azureResourceMetrics{metrics: []string{name}}
	}
}

func (s *azureScraper) getResourceMetricsValues(ctx context.Context, resourceID string) {
	res := *s.resources[resourceID]

	for compositeKey, metricsByGrain := range res.metricsByCompositeKey {

		if time.Since(metricsByGrain.metricsValuesUpdated).Seconds() < float64(timeGrains[compositeKey.timeGrain]) {
			continue
		}
		metricsByGrain.metricsValuesUpdated = time.Now()

		start := 0

		for start < len(metricsByGrain.metrics) {

			end := start + s.cfg.MaximumNumberOfMetricsInACall
			if end > len(metricsByGrain.metrics) {
				end = len(metricsByGrain.metrics)
			}

			opts := getResourceMetricsValuesRequestOptions(
				metricsByGrain.metrics,
				compositeKey.dimensions,
				compositeKey.timeGrain,
				start,
				end,
			)
			start = end

			result, err := s.clientMetricsValues.List(
				ctx,
				resourceID,
				&opts,
			)
			if err != nil {
				s.settings.Logger.Error("failed to get Azure Metrics values data", zap.Error(err))
				return
			}

			for _, metric := range result.Value {

				for _, timeseriesElement := range metric.Timeseries {
					if timeseriesElement.Data != nil {
						attributes := map[string]*string{}
						for _, value := range timeseriesElement.Metadatavalues {
							name := metadataPrefix + *value.Name.Value
							attributes[name] = value.Value
						}
						if len(res.location) > 0 {
							attributes[location] = &res.location
						}
						if s.cfg.AppendTagsAsAttributes {
							for tagName, value := range res.tags {
								name := tagPrefix + tagName
								attributes[name] = value
							}
						}
						for _, metricValue := range timeseriesElement.Data {
							s.processTimeseriesData(resourceID, metric, metricValue, attributes)
						}
					}
				}
			}
		}
	}
}

func getResourceMetricsValuesRequestOptions(
	metrics []string,
	dimensionsStr string,
	timeGrain string,
	start int,
	end int,
) armmonitor.MetricsClientListOptions {
	resType := strings.Join(metrics[start:end], ",")
	filter := armmonitor.MetricsClientListOptions{
		Metricnames: &resType,
		Interval:    to.Ptr(timeGrain),
		Timespan:    to.Ptr(timeGrain),
		Aggregation: to.Ptr(strings.Join(aggregations, ",")),
	}

	if len(dimensionsStr) > 0 {
		var dimensionsFilter bytes.Buffer
		dimensions := strings.Split(dimensionsStr, ",")
		for i, dimension := range dimensions {
			dimensionsFilter.WriteString(dimension)
			dimensionsFilter.WriteString(" eq '*' ")
			if i < len(dimensions)-1 {
				dimensionsFilter.WriteString(" and ")
			}
		}
		dimensionFilterString := dimensionsFilter.String()
		filter.Filter = &dimensionFilterString
	}

	return filter
}

func (s *azureScraper) processTimeseriesData(
	resourceID string,
	metric *armmonitor.Metric,
	metricValue *armmonitor.MetricValue,
	attributes map[string]*string,
) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	ts := pcommon.NewTimestampFromTime(time.Now())

	aggregationsData := []struct {
		name  string
		value *float64
	}{
		{"Average", metricValue.Average},
		{"Count", metricValue.Count},
		{"Maximum", metricValue.Maximum},
		{"Minimum", metricValue.Minimum},
		{"Total", metricValue.Total},
	}
	for _, aggregation := range aggregationsData {
		if aggregation.value != nil {
			s.mb.AddDataPoint(
				resourceID,
				*metric.Name.Value,
				aggregation.name,
				string(*metric.Unit),
				attributes,
				ts,
				*aggregation.value,
			)
		}
	}
}
