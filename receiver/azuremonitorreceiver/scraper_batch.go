// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
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
		armMonitorDefinitionsClientFunc: armmonitor.NewMetricDefinitionsClient,
		azQueryMetricsBatchClientFunc:   azquery.NewMetricsBatchClient,
		mutex:                           &sync.Mutex{},
	}
}

type ArmsubscriptionClient interface {
	NewListPager(options *armsubscriptions.ClientListOptions) *runtime.Pager[armsubscriptions.ClientListResponse]
	NewListLocationsPager(subscriptionID string, options *armsubscriptions.ClientListLocationsOptions) *runtime.Pager[armsubscriptions.ClientListLocationsResponse]
}

type azureBatchScraper struct {
	cred                             azcore.TokenCredential
	cfg                              *Config
	settings                         component.TelemetrySettings
	discoveredSubscriptions          map[string]*armsubscriptions.Subscription
	regionsFromSubscriptions         map[string]map[string]struct{}
	resources                        map[string]map[string]*azureResource
	resourceTypes                    map[string]map[string]*azureType
	resourcesUpdated                 time.Time
	mb                               *metadata.MetricsBuilder
	azIDCredentialsFunc              func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	azIDWorkloadFunc                 func(options *azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error)
	armClientOptions                 *arm.ClientOptions
	armSubscriptionclient            ArmsubscriptionClient
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

func (s *azureBatchScraper) getArmsubscriptionClient() ArmsubscriptionClient {
	client, _ := armsubscriptions.NewClient(s.cred, s.armClientOptions)
	return client
}

func (s *azureBatchScraper) getArmClient(subscriptionId string) ArmClient {
	client, _ := armresources.NewClient(subscriptionId, s.cred, s.armClientOptions)
	return client
}

func (s *azureBatchScraper) getMetricsDefinitionsClient(subscriptionId string) MetricsDefinitionsClientInterface {
	client, _ := s.armMonitorDefinitionsClientFunc(subscriptionId, s.cred, s.armClientOptions)
	return client
}

type MetricBatchValuesClient interface {
	QueryBatch(ctx context.Context, subscriptionID string, metricNamespace string, metricNames []string, resourceIDs azquery.ResourceIDList, options *azquery.MetricsBatchClientQueryBatchOptions) (
		azquery.MetricsBatchClientQueryBatchResponse, error,
	)
}

func (s *azureBatchScraper) GetMetricsBatchValuesClient(region string) MetricBatchValuesClient {
	endpoint := "https://" + region + ".metrics.monitor.azure.com"
	s.settings.Logger.Info("Batch Endpoint", zap.String("endpoint", endpoint))
	client, _ := azquery.NewMetricsBatchClient(endpoint, s.cred, s.azQueryMetricsBatchClientOptions)
	return client
}

func (s *azureBatchScraper) start(ctx context.Context, _ component.Host) (err error) {
	if err = s.loadCredentials(); err != nil {
		return err
	}

	s.armClientOptions = s.getArmClientOptions()
	s.azQueryMetricsBatchClientOptions = s.getAzQueryMetricsBatchClientOptions()
	s.armSubscriptionclient = s.getArmsubscriptionClient()
	s.resources = map[string]map[string]*azureResource{}
	s.resourceTypes = map[string]map[string]*azureType{}
	s.discoveredSubscriptions = map[string]*armsubscriptions.Subscription{}
	s.regionsFromSubscriptions = map[string]map[string]struct{}{}

	if !s.cfg.DiscoverSubscription {
		s.resources[s.cfg.SubscriptionID] = make(map[string]*azureResource)
		s.resourceTypes[s.cfg.SubscriptionID] = make(map[string]*azureType)
		s.discoveredSubscriptions[s.cfg.SubscriptionID] = &armsubscriptions.Subscription{
			ID:          &s.cfg.SubscriptionID,
			DisplayName: &s.cfg.SubscriptionID,
		}
	} else {
		s.getSubscriptions(ctx)
	}

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
	if !(time.Since(s.resourcesUpdated).Seconds() < s.cfg.CacheResources) {
		s.getSubscriptions(ctx)
	}
	var wg sync.WaitGroup
	for _, subscription := range s.discoveredSubscriptions {
		wg.Add(1)
		go func(subscription *armsubscriptions.Subscription) {
			defer wg.Done()

			s.getResources(ctx, *subscription.SubscriptionID)
			resourceTypesWithDefinitions := make(chan string)
			go func() {
				defer close(resourceTypesWithDefinitions)
				for resourceType := range s.resourceTypes[*subscription.SubscriptionID] {
					s.getResourceMetricsDefinitionsByType(ctx, subscription, resourceType)
					resourceTypesWithDefinitions <- resourceType
				}
			}()

			var wg2 sync.WaitGroup
			for resourceType := range resourceTypesWithDefinitions {
				wg2.Add(1)
				go func(subscription *armsubscriptions.Subscription, resourceType string) {
					defer wg2.Done()
					s.getBatchMetricsValues(ctx, subscription, resourceType)
				}(subscription, resourceType)
			}

			wg2.Wait()
		}(subscription)

	}

	wg.Wait()
	return s.mb.Emit(), nil
}

func (s *azureBatchScraper) getSubscriptions(ctx context.Context) {
	opts := &armsubscriptions.ClientListOptions{}
	pager := s.armSubscriptionclient.NewListPager(opts)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Subscriptions", zap.Error(err))
			return
		}

		for _, subscription := range nextResult.Value {
			s.resources[*subscription.SubscriptionID] = make(map[string]*azureResource)
			s.resourceTypes[*subscription.SubscriptionID] = make(map[string]*azureType)
			s.discoveredSubscriptions[*subscription.SubscriptionID] = subscription
			s.regionsFromSubscriptions[*subscription.SubscriptionID] = make(map[string]struct{})
		}
	}
}

func (s *azureBatchScraper) getResources(ctx context.Context, subscriptionId string) {
	if time.Since(s.resourcesUpdated).Seconds() < s.cfg.CacheResources {
		return
	}
	clientResources := s.getArmClient(subscriptionId)

	existingResources := map[string]void{}
	for id := range s.resources[subscriptionId] {
		existingResources[id] = void{}
	}

	filter := s.getResourcesFilter()
	opts := &armresources.ClientListOptions{
		Filter: &filter,
	}

	updatedTypes := map[string]*azureType{}
	pager := clientResources.NewListPager(opts)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Resources data", zap.Error(err))
			return
		}

		for _, resource := range nextResult.Value {

			if _, ok := s.resources[subscriptionId][*resource.ID]; !ok {
				resourceGroup := getResourceGroupFromID(*resource.ID)
				attributes := map[string]*string{
					attributeName:          resource.Name,
					attributeResourceGroup: &resourceGroup,
					attributeResourceType:  resource.Type,
				}

				if resource.Location != nil {
					s.regionsFromSubscriptions[subscriptionId][*resource.Location] = struct{}{}
					attributes[attributeLocation] = resource.Location
				}

				s.resources[subscriptionId][*resource.ID] = &azureResource{
					attributes: attributes,
					tags:       resource.Tags,
				}

				if updatedTypes[*resource.Type] == nil {
					updatedTypes[*resource.Type] = &azureType{
						name:        resource.Type,
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
			delete(s.resources[subscriptionId], idToDelete)
		}
	}

	s.resourcesUpdated = time.Now()
	maps.Copy(s.resourceTypes[subscriptionId], updatedTypes)
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

func (s *azureBatchScraper) getResourceMetricsDefinitionsByType(ctx context.Context, subscription *armsubscriptions.Subscription, resourceType string) {

	if time.Since(s.resourceTypes[*subscription.SubscriptionID][resourceType].metricsDefinitionsUpdated).Seconds() < s.cfg.CacheResourcesDefinitions {
		return
	}

	s.resourceTypes[*subscription.SubscriptionID][resourceType].metricsByCompositeKey = map[metricsCompositeKey]*azureResourceMetrics{}

	resourceIds := s.resourceTypes[*subscription.SubscriptionID][resourceType].resourceIds
	if len(resourceIds) == 0 && resourceIds[0] != nil {
		return
	}

	clientMetricsDefinitions := s.getMetricsDefinitionsClient(*subscription.SubscriptionID)
	pager := clientMetricsDefinitions.NewListPager(*resourceIds[0], nil)
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
			s.storeMetricsDefinitionByType(*subscription.SubscriptionID, resourceType, name, compositeKey)
		}
	}
	s.resourceTypes[*subscription.SubscriptionID][resourceType].metricsDefinitionsUpdated = time.Now()
}

func (s *azureBatchScraper) storeMetricsDefinitionByType(subscriptionid string, resourceType string, name string, compositeKey metricsCompositeKey) {
	if _, ok := s.resourceTypes[subscriptionid][resourceType].metricsByCompositeKey[compositeKey]; ok {
		s.resourceTypes[subscriptionid][resourceType].metricsByCompositeKey[compositeKey].metrics = append(
			s.resourceTypes[subscriptionid][resourceType].metricsByCompositeKey[compositeKey].metrics, name,
		)
	} else {
		s.resourceTypes[subscriptionid][resourceType].metricsByCompositeKey[compositeKey] = &azureResourceMetrics{metrics: []string{name}}
	}
}

func (s *azureBatchScraper) getBatchMetricsValues(ctx context.Context, subscription *armsubscriptions.Subscription, resourceType string) {
	resType := *s.resourceTypes[*subscription.SubscriptionID][resourceType]

	for compositeKey, metricsByGrain := range resType.metricsByCompositeKey {

		if time.Since(metricsByGrain.metricsValuesUpdated).Seconds() < float64(timeGrains[compositeKey.timeGrain]) {
			continue
		}

		now := time.Now().UTC()
		metricsByGrain.metricsValuesUpdated = now
		startTime := now.Add(time.Duration(-timeGrains[compositeKey.timeGrain]) * time.Second * 2) // times 2 because for some resources, data are missing for the very latest timestamp

		start := 0
		for start < len(metricsByGrain.metrics) {

			end := start + s.cfg.MaximumNumberOfMetricsInACall
			if end > len(metricsByGrain.metrics) {
				end = len(metricsByGrain.metrics)
			}
			for region := range s.regionsFromSubscriptions[*subscription.SubscriptionID] {
				clientMetrics := s.GetMetricsBatchValuesClient(region)
				s.settings.Logger.Info("scrape", zap.String("subscription", *subscription.DisplayName), zap.String("resourceType", resourceType), zap.String("region", region))
				response, err := clientMetrics.QueryBatch(
					ctx,
					*subscription.SubscriptionID,
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
					s.settings.Logger.Error("failed to get Azure Metrics values data", zap.String("subscription", *subscription.SubscriptionID), zap.String("region", region), zap.String("resourceType", resourceType), zap.Error(err))
					return
				}

				start = end
				for _, metricValues := range response.Values {
					for _, metric := range metricValues.Values {

						for _, timeseriesElement := range metric.TimeSeries {
							if timeseriesElement.Data != nil {
								if metricValues.ResourceID != nil {
									res := s.resources[*subscription.SubscriptionID][*metricValues.ResourceID]
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
									attributes["subscription"] = subscription.DisplayName
									attributes["timegrain"] = &compositeKey.timeGrain
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

	ts := pcommon.NewTimestampFromTime(*metricValue.TimeStamp)

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
