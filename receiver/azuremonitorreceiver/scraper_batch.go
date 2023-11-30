// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

type azureType struct {
	name                      *string
	attributes                map[string]*string
	resourceIds               []*string
	metricsByCompositeKey     map[metricsCompositeKey]*azureResourceMetrics
	metricsDefinitionsUpdated time.Time
}

func newBatchScraper(conf *Config, settings receiver.CreateSettings) *azureBatchScraper {
	return &azureBatchScraper{
		cfg:                             conf,
		settings:                        settings.TelemetrySettings,
		mb:                              metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		azIDCredentialsFunc:             azidentity.NewClientSecretCredential,
		azIDWorkloadFunc:                azidentity.NewWorkloadIdentityCredential,
		armClientFunc:                   armresources.NewClient,
		armMonitorDefinitionsClientFunc: armmonitor.NewMetricDefinitionsClient,
		azQueryMetricsBatchClientFunc:   azquery.NewMetricsBatchClient,
		mutex:                           &sync.Mutex{},
	}
}

type azureBatchScraper struct {
	cred azcore.TokenCredential

	clientResources          ArmClient
	clientMetricsDefinitions MetricsDefinitionsClientInterface
	clientMetricsBatchValues MetricBatchValuesClient

	cfg                              *Config
	settings                         component.TelemetrySettings
	resources                        map[string]*azureResource
	resourceTypes                    map[string]*azureType
	resourcesUpdated                 time.Time
	mb                               *metadata.MetricsBuilder
	azIDCredentialsFunc              func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	azIDWorkloadFunc                 func(options *azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error)
	armClientOptions                 *arm.ClientOptions
	armClientFunc                    func(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error)
	armMonitorDefinitionsClientFunc  func(string, azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error)
	azQueryMetricsBatchClientOptions *azquery.MetricsBatchClientOptions
	azQueryMetricsBatchClientFunc    func(string, azcore.TokenCredential, *azquery.MetricsBatchClientOptions) (*azquery.MetricsBatchClient, error)
	mutex                            *sync.Mutex
}

func (s *azureBatchScraper) getArmClientOptions() *arm.ClientOptions {
	var cloudToUse cloud.Configuration
	switch s.cfg.Cloud {
	case azureGovernmentCloud:
		cloudToUse = cloud.AzureGovernment
	default:
		cloudToUse = cloud.AzurePublic
	}
	options := arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cloudToUse,
		},
	}

	return &options
}

func (s *azureBatchScraper) getAzQueryMetricsBatchClientOptions() *azquery.MetricsBatchClientOptions {
	var cloudToUse cloud.Configuration
	switch s.cfg.Cloud {
	case azureGovernmentCloud:
		cloudToUse = cloud.AzureGovernment
	default:
		cloudToUse = cloud.AzurePublic
	}

	options := azquery.MetricsBatchClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cloudToUse,
		},
	}

	return &options
}

func (s *azureBatchScraper) getArmClient() ArmClient {
	client, _ := s.armClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
	return client
}

func (s *azureBatchScraper) getMetricsDefinitionsClient() MetricsDefinitionsClientInterface {
	client, _ := s.armMonitorDefinitionsClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
	return client
}

type MetricBatchValuesClient interface {
	QueryBatch(ctx context.Context, subscriptionID string, metricNamespace string, metricNames []string, resourceIDs azquery.ResourceIDList, options *azquery.MetricsBatchClientQueryBatchOptions) (
		azquery.MetricsBatchClientQueryBatchResponse, error,
	)
}

func (s *azureBatchScraper) GetMetricsBatchValuesClient() MetricBatchValuesClient {
	endpoint := "https://" + s.cfg.Region + ".metrics.monitor.azure.com"
	s.settings.Logger.Info("Batch Endpoint", zap.String("endpoint", endpoint))
	client, _ := azquery.NewMetricsBatchClient(endpoint, s.cred, s.azQueryMetricsBatchClientOptions)
	return client
}

func (s *azureBatchScraper) start(_ context.Context, _ component.Host) (err error) {
	if err = s.loadCredentials(); err != nil {
		return err
	}

	s.armClientOptions = s.getArmClientOptions()
	s.clientResources = s.getArmClient()
	s.clientMetricsDefinitions = s.getMetricsDefinitionsClient()
	s.azQueryMetricsBatchClientOptions = s.getAzQueryMetricsBatchClientOptions()
	s.clientMetricsBatchValues = s.GetMetricsBatchValuesClient()

	s.resources = map[string]*azureResource{}
	s.resourceTypes = map[string]*azureType{}

	return
}

func (s *azureBatchScraper) loadCredentials() (err error) {
	switch s.cfg.Authentication {
	case servicePrincipal:
		if s.cred, err = s.azIDCredentialsFunc(s.cfg.TenantID, s.cfg.ClientID, s.cfg.ClientSecret, nil); err != nil {
			return err
		}
	case workloadIdentity:
		if s.cred, err = s.azIDWorkloadFunc(nil); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown authentication %v", s.cfg.Authentication)
	}
	return nil
}

func (s *azureBatchScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {

	s.getResources(ctx)
	s.settings.Logger.Info("scrape", zap.Any("s.resourceTypes", s.resourceTypes))

	resourceTypesWithDefinitions := make(chan string)
	go func() {
		defer close(resourceTypesWithDefinitions)
		for resourceType := range s.resourceTypes {
			s.getResourceMetricsDefinitionsByType(ctx, resourceType)
			resourceTypesWithDefinitions <- resourceType
		}
	}()

	var wg sync.WaitGroup
	for resourceType := range resourceTypesWithDefinitions {
		wg.Add(1)
		go func(resourceType string) {
			defer wg.Done()
			s.settings.Logger.Info("scrape", zap.String("resourceType", resourceType))
			s.settings.Logger.Info("scrape", zap.Any("s.resourceTypes[resourceType]", *s.resourceTypes[resourceType]))
			s.getBatchMetricsValues(ctx, resourceType)
		}(resourceType)
	}
	wg.Wait()

	return s.mb.Emit(
		metadata.WithAzureMonitorSubscriptionID(s.cfg.SubscriptionID),
		metadata.WithAzureMonitorTenantID(s.cfg.TenantID),
	), nil
}

func (s *azureBatchScraper) getResources(ctx context.Context) {
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

	updatedTypes := map[string]*azureType{}
	pager := s.clientResources.NewListPager(opts)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Resources data", zap.Error(err))
			return
		}

		for _, resource := range nextResult.Value {
			if _, ok := s.resources[*resource.ID]; !ok {
				resourceGroup := getResourceGroupFromID(*resource.ID)
				attributes := map[string]*string{
					attributeName:          resource.Name,
					attributeResourceGroup: &resourceGroup,
					attributeResourceType:  resource.Type,
				}

				if resource.Location != nil {
					attributes[attributeLocation] = resource.Location
				}

				s.resources[*resource.ID] = &azureResource{
					attributes: attributes,
					tags:       resource.Tags,
				}

				if updatedTypes[*resource.Type] == nil {
					updatedTypes[*resource.Type] = &azureType{
						name:        resource.Name,
						attributes:  map[string]*string{},
						resourceIds: []*string{resource.ID},
					}
				} else {
					updatedTypes[*resource.Type].resourceIds = append(updatedTypes[*resource.Type].resourceIds, resource.ID)
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
	s.resourceTypes = updatedTypes
}

func (s *azureBatchScraper) getResourcesFilter() string {
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

func (s *azureBatchScraper) getResourceMetricsDefinitionsByType(ctx context.Context, resourceType string) {

	if time.Since(s.resourceTypes[resourceType].metricsDefinitionsUpdated).Seconds() < s.cfg.CacheResourcesDefinitions {
		return
	}

	s.resourceTypes[resourceType].metricsByCompositeKey = map[metricsCompositeKey]*azureResourceMetrics{}

	resourceIds := s.resourceTypes[resourceType].resourceIds
	if len(resourceIds) == 0 && resourceIds[0] != nil {
		return
	}

	pager := s.clientMetricsDefinitions.NewListPager(*resourceIds[0], nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Metrics definitions data", zap.Error(err))
			return
		}

		for _, v := range nextResult.Value {
			s.settings.Logger.Info("getResourceMetricsDefinitionsByType", zap.String("resourceType", resourceType), zap.Any("v", v))
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
				compositeKey.dimensions = strings.Join(dimensionsSlice, ",")
			}
			s.storeMetricsDefinitionByType(resourceType, name, compositeKey)
		}
	}
	s.resourceTypes[resourceType].metricsDefinitionsUpdated = time.Now()
}

func (s *azureBatchScraper) storeMetricsDefinitionByType(resourceType string, name string, compositeKey metricsCompositeKey) {
	if _, ok := s.resourceTypes[resourceType].metricsByCompositeKey[compositeKey]; ok {
		s.resourceTypes[resourceType].metricsByCompositeKey[compositeKey].metrics = append(
			s.resourceTypes[resourceType].metricsByCompositeKey[compositeKey].metrics, name,
		)
	} else {
		s.resourceTypes[resourceType].metricsByCompositeKey[compositeKey] = &azureResourceMetrics{metrics: []string{name}}
	}
}

func (s *azureBatchScraper) getBatchMetricsValues(ctx context.Context, resourceType string) {
	resType := *s.resourceTypes[resourceType]

	for compositeKey, metricsByGrain := range resType.metricsByCompositeKey {

		if time.Since(metricsByGrain.metricsValuesUpdated).Seconds() < float64(timeGrains[compositeKey.timeGrain]) {
			continue
		}

		now := time.Now().UTC()
		metricsByGrain.metricsValuesUpdated = now
		startTime := now.Add(time.Duration(-timeGrains[compositeKey.timeGrain]) * time.Second)
		s.settings.Logger.Info("getBatchMetricsValues", zap.String("resourceType", resourceType), zap.Any("metricNames", metricsByGrain.metrics), zap.Any("startTime", startTime), zap.Any("now", now), zap.String("timeGrain", compositeKey.timeGrain))

		start := 0
		for start < len(metricsByGrain.metrics) {

			end := start + s.cfg.MaximumNumberOfMetricsInACall
			if end > len(metricsByGrain.metrics) {
				end = len(metricsByGrain.metrics)
			}

			response, err := s.clientMetricsBatchValues.QueryBatch(
				ctx,
				s.cfg.SubscriptionID,
				resourceType,
				metricsByGrain.metrics[start:end],
				azquery.ResourceIDList{ResourceIDs: resType.resourceIds},
				&azquery.MetricsBatchClientQueryBatchOptions{
					Aggregation: to.SliceOfPtrs(
						azquery.AggregationTypeAverage,
						azquery.AggregationTypeMaximum,
						azquery.AggregationTypeMinimum,
						azquery.AggregationTypeTotal,
						azquery.AggregationTypeCount,
					),
					StartTime: to.Ptr(startTime.Format(time.RFC3339)),
					EndTime:   to.Ptr(now.Format(time.RFC3339)),
					Interval:  to.Ptr(compositeKey.timeGrain),
					Top:       to.Ptr(int32(s.cfg.MaximumNumberOfDimensionsInACall)), // Defaults to 10 (may be limiting results)
				},
			)

			if err != nil {
				s.settings.Logger.Error("failed to get Azure Metrics values data", zap.Error(err))
				return
			}

			start = end
			for _, metricValues := range response.Values {
				for _, metric := range metricValues.Values {
					for _, timeseriesElement := range metric.TimeSeries {

						if timeseriesElement.Data != nil {
							res := s.resources[*metricValues.ResourceID]
							attributes := map[string]*string{}
							for name, value := range res.attributes {
								attributes[name] = value
							}
							for _, value := range timeseriesElement.MetadataValues {
								name := metadataPrefix + *value.Name.Value
								attributes[name] = value.Value
							}
							if s.cfg.AppendTagsAsAttributes {
								for tagName, value := range res.tags {
									name := tagPrefix + tagName
									attributes[name] = value
								}
							}
							for _, metricValue := range timeseriesElement.Data {
								s.processQueryTimeseriesData(*metricValues.ResourceID, metric, metricValue, attributes)
							}
						}
					}
				}
			}
		}
	}
}

func (s *azureBatchScraper) processQueryTimeseriesData(
	resourceID string,
	metric *azquery.Metric,
	metricValue *azquery.MetricValue,
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
