// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v2"
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
	resourceIDs               []string
	metricsByCompositeKey     map[metricsCompositeKey]*azureResourceMetrics
	metricsDefinitionsUpdated time.Time
}

func newBatchScraper(conf *Config, settings receiver.Settings) *azureBatchScraper {
	return &azureBatchScraper{
		cfg:                      conf,
		settings:                 settings.TelemetrySettings,
		mb:                       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		azDefaultCredentialsFunc: azidentity.NewDefaultAzureCredential,
		azIDCredentialsFunc:      azidentity.NewClientSecretCredential,
		azIDWorkloadFunc:         azidentity.NewWorkloadIdentityCredential,
		azManagedIdentityFunc:    azidentity.NewManagedIdentityCredential,
		mutex:                    &sync.Mutex{},
		time:                     &timeWrapper{},
		clientOptionsResolver:    newClientOptionsResolver(conf.Cloud),
	}
}

type azureBatchScraper struct {
	cred     azcore.TokenCredential
	cfg      *Config
	settings component.TelemetrySettings
	// resources on which we'll get attributes. Stored by resource id and subscription id.
	resources map[string]map[string]*azureResource
	// resourceTypes on which we'll collect metrics. Stored by resource type and subscription id.
	resourceTypes map[string]map[string]*azureType
	// subscriptions on which we'll look up resources. Stored by subscription id.
	subscriptions        map[string]*azureSubscription
	subscriptionsUpdated time.Time
	// regions on which we'll collect metrics. Stored by subscription id.
	regions                  map[string]map[string]struct{}
	mb                       *metadata.MetricsBuilder
	azDefaultCredentialsFunc func(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error)
	azIDCredentialsFunc      func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	azIDWorkloadFunc         func(options *azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error)
	azManagedIdentityFunc    func(options *azidentity.ManagedIdentityCredentialOptions) (*azidentity.ManagedIdentityCredential, error)

	mutex                 *sync.Mutex
	time                  timeNowIface
	clientOptionsResolver ClientOptionsResolver
}

func (s *azureBatchScraper) GetMetricsBatchValuesClient(region string) (*azmetrics.Client, error) {
	endpoint := "https://" + region + ".metrics.monitor.azure.com"
	s.settings.Logger.Info("Batch Endpoint", zap.String("endpoint", endpoint))
	return azmetrics.NewClient(endpoint, s.cred, s.clientOptionsResolver.GetAzMetricsClientOptions())
}

func (s *azureBatchScraper) start(_ context.Context, _ component.Host) (err error) {
	if err = s.loadCredentials(); err != nil {
		return err
	}

	s.subscriptions = map[string]*azureSubscription{}
	s.resourceTypes = map[string]map[string]*azureType{}
	s.resources = map[string]map[string]*azureResource{}
	s.regions = map[string]map[string]struct{}{}

	return
}

func (s *azureBatchScraper) loadSubscription(sub azureSubscription) {
	s.resourceTypes[sub.SubscriptionID] = make(map[string]*azureType)
	s.resources[sub.SubscriptionID] = make(map[string]*azureResource)
	s.regions[sub.SubscriptionID] = make(map[string]struct{})
	s.subscriptions[sub.SubscriptionID] = &azureSubscription{
		SubscriptionID: sub.SubscriptionID,
		DisplayName:    sub.DisplayName,
	}
}

func (s *azureBatchScraper) unloadSubscription(id string) {
	delete(s.subscriptions, id)
	delete(s.resourceTypes, id)
	delete(s.resources, id)
	delete(s.regions, id)
}

// TODO: duplicate
func (s *azureBatchScraper) loadCredentials() (err error) {
	switch s.cfg.Credentials {
	case defaultCredentials:
		if s.cred, err = s.azDefaultCredentialsFunc(nil); err != nil {
			return err
		}
	case servicePrincipal:
		if s.cred, err = s.azIDCredentialsFunc(s.cfg.TenantID, s.cfg.ClientID, s.cfg.ClientSecret, nil); err != nil {
			return err
		}
	case workloadIdentity:
		if s.cred, err = s.azIDWorkloadFunc(nil); err != nil {
			return err
		}
	case managedIdentity:
		var options *azidentity.ManagedIdentityCredentialOptions
		if s.cfg.ClientID != "" {
			options = &azidentity.ManagedIdentityCredentialOptions{
				ID: azidentity.ClientID(s.cfg.ClientID),
			}
		}
		if s.cred, err = s.azManagedIdentityFunc(options); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown credentials %v", s.cfg.Credentials)
	}
	return nil
}

func (s *azureBatchScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.getSubscriptions(ctx)

	for subscriptionID, subscription := range s.subscriptions {
		s.getResourcesAndTypes(ctx, subscriptionID)

		resourceTypesWithDefinitions := make(chan string)
		go func(subscriptionID string) {
			defer close(resourceTypesWithDefinitions)
			for resourceType := range s.resourceTypes[subscriptionID] {
				s.getResourceMetricsDefinitionsByType(ctx, subscriptionID, resourceType)
				resourceTypesWithDefinitions <- resourceType
			}
		}(subscriptionID)

		var wg sync.WaitGroup
		for resourceType := range resourceTypesWithDefinitions {
			wg.Add(1)
			go func(subscriptionID, resourceType string) {
				defer wg.Done()
				s.getBatchMetricsValues(ctx, subscriptionID, resourceType)
			}(subscriptionID, resourceType)
		}

		wg.Wait()

		// Once all metrics has been collected for one subscription, we move to the next.
		// We need to keep it synchronous to have the subscription id in resource attributes and not metrics attributes.
		// It can be revamped later if we need to parallelize more, but currently, resource emit is not thread safe.
		rb := s.mb.NewResourceBuilder()
		rb.SetAzuremonitorTenantID(s.cfg.TenantID)
		rb.SetAzuremonitorSubscriptionID(subscriptionID)
		rb.SetAzuremonitorSubscription(subscription.DisplayName)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
	return s.mb.Emit(), nil
}

// TODO: duplicate
func (s *azureBatchScraper) getSubscriptions(ctx context.Context) {
	if time.Since(s.subscriptionsUpdated).Seconds() < s.cfg.CacheResources {
		return
	}

	// if subscriptions discovery is enabled, we'll need a client
	armSubscriptionClient, clientErr := armsubscriptions.NewClient(s.cred, s.clientOptionsResolver.GetArmSubscriptionsClientOptions())
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Subscriptions", zap.Error(clientErr))
		return
	}

	// Make a special case for when we only have subscription ids configured (discovery disabled)
	if !s.cfg.DiscoverSubscriptions {
		for _, subID := range s.cfg.SubscriptionIDs {
			// we don't need additional info,
			// => It simply load the subscription id
			if !s.cfg.MetricsBuilderConfig.ResourceAttributes.AzuremonitorSubscription.Enabled {
				s.loadSubscription(azureSubscription{
					SubscriptionID: subID,
				})
				continue
			}

			// We need additional info,
			// => It makes some get requests
			resp, err := armSubscriptionClient.Get(ctx, subID, &armsubscriptions.ClientGetOptions{})
			if err != nil {
				s.settings.Logger.Error("failed to get Azure Subscription", zap.String("subscription_id", subID), zap.Error(err))
				return
			}
			s.loadSubscription(azureSubscription{
				SubscriptionID: *resp.SubscriptionID,
				DisplayName:    *resp.DisplayName,
			})
		}
		s.subscriptionsUpdated = time.Now()
		return
	}

	opts := &armsubscriptions.ClientListOptions{}
	pager := armSubscriptionClient.NewListPager(opts)

	existingSubscriptions := map[string]void{}
	for id := range s.subscriptions {
		existingSubscriptions[id] = void{}
	}

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Subscriptions", zap.Error(err))
			return
		}

		for _, subscription := range nextResult.Value {
			s.loadSubscription(azureSubscription{
				SubscriptionID: *subscription.SubscriptionID,
				DisplayName:    *subscription.DisplayName,
			})
			delete(existingSubscriptions, *subscription.SubscriptionID)
		}
	}
	if len(existingSubscriptions) > 0 {
		for idToDelete := range existingSubscriptions {
			s.unloadSubscription(idToDelete)
		}
	}

	s.subscriptionsUpdated = time.Now()
}

// TODO: partially duplicate
func (s *azureBatchScraper) getResourcesAndTypes(ctx context.Context, subscriptionID string) {
	if time.Since(s.subscriptions[subscriptionID].resourcesUpdated).Seconds() < s.cfg.CacheResources {
		return
	}
	clientResources, clientErr := armresources.NewClient(subscriptionID, s.cred, s.clientOptionsResolver.GetArmResourceClientOptions(subscriptionID))
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Resources", zap.Error(clientErr))
		return
	}

	existingResources := map[string]void{}
	for id := range s.resources[subscriptionID] {
		existingResources[id] = void{}
	}

	filter := s.getResourcesFilter()
	opts := &armresources.ClientListOptions{
		Filter: &filter,
	}

	resourceTypes := map[string]*azureType{}
	pager := clientResources.NewListPager(opts)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Resources data", zap.Error(err))
			return
		}
		for _, resource := range nextResult.Value {
			if _, ok := s.resources[subscriptionID][*resource.ID]; !ok {
				resourceGroup := getResourceGroupFromID(*resource.ID)
				attributes := map[string]*string{
					attributeName:          resource.Name,
					attributeResourceGroup: &resourceGroup,
					attributeResourceType:  resource.Type,
				}
				if resource.Location != nil {
					s.regions[subscriptionID][*resource.Location] = struct{}{}
					attributes[attributeLocation] = resource.Location
				}
				s.resources[subscriptionID][*resource.ID] = &azureResource{
					attributes:   attributes,
					tags:         resource.Tags,
					resourceType: resource.Type,
				}
				if resourceTypes[*resource.Type] == nil {
					resourceTypes[*resource.Type] = &azureType{
						name:        resource.Type,
						resourceIDs: []string{*resource.ID},
					}
				} else {
					resourceTypes[*resource.Type].resourceIDs = append(resourceTypes[*resource.Type].resourceIDs, *resource.ID)
				}
			}
			delete(existingResources, *resource.ID)
		}
	}
	if len(existingResources) > 0 {
		for idToDelete := range existingResources {
			delete(s.resources[subscriptionID], idToDelete)
		}
	}

	s.subscriptions[subscriptionID].resourcesUpdated = time.Now()
	maps.Copy(s.resourceTypes[subscriptionID], resourceTypes)
}

// TODO: duplicate
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

// TODO: Partially duplicate
func (s *azureBatchScraper) getResourceMetricsDefinitionsByType(ctx context.Context, subscriptionID, resourceType string) {
	if time.Since(s.resourceTypes[subscriptionID][resourceType].metricsDefinitionsUpdated).Seconds() < s.cfg.CacheResourcesDefinitions {
		return
	}

	clientMetricsDefinitions, clientErr := armmonitor.NewMetricDefinitionsClient(subscriptionID, s.cred, s.clientOptionsResolver.GetArmMonitorClientOptions())
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Metrics definitions", zap.Error(clientErr))
		return
	}

	s.resourceTypes[subscriptionID][resourceType].metricsByCompositeKey = map[metricsCompositeKey]*azureResourceMetrics{}

	resourceIDs := s.resourceTypes[subscriptionID][resourceType].resourceIDs
	if len(resourceIDs) == 0 && len(resourceIDs[0]) > 0 {
		return
	}

	pager := clientMetricsDefinitions.NewListPager(resourceIDs[0], nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Metrics definitions data", zap.Error(err))
			return
		}

		for _, v := range nextResult.Value {
			metricName := *v.Name.Value
			metricAggregations := getMetricAggregations(*v.Namespace, metricName, s.cfg.Metrics)
			if len(metricAggregations) == 0 {
				continue
			}

			timeGrain := *v.MetricAvailabilities[0].TimeGrain
			dimensions := filterDimensions(v.Dimensions, s.cfg.Dimensions, resourceType, metricName)
			compositeKey := metricsCompositeKey{
				timeGrain:    timeGrain,
				dimensions:   serializeDimensions(dimensions),
				aggregations: strings.Join(metricAggregations, ","),
			}
			s.storeMetricsDefinitionByType(subscriptionID, resourceType, metricName, compositeKey)
		}
	}
	s.resourceTypes[subscriptionID][resourceType].metricsDefinitionsUpdated = time.Now()
}

// TODO: duplicate
func (s *azureBatchScraper) storeMetricsDefinitionByType(subscriptionID string, resourceType string, name string, compositeKey metricsCompositeKey) {
	if _, ok := s.resourceTypes[subscriptionID][resourceType].metricsByCompositeKey[compositeKey]; ok {
		s.resourceTypes[subscriptionID][resourceType].metricsByCompositeKey[compositeKey].metrics = append(
			s.resourceTypes[subscriptionID][resourceType].metricsByCompositeKey[compositeKey].metrics, name,
		)
	} else {
		s.resourceTypes[subscriptionID][resourceType].metricsByCompositeKey[compositeKey] = &azureResourceMetrics{metrics: []string{name}}
	}
}

func (s *azureBatchScraper) getBatchMetricsValues(ctx context.Context, subscriptionID, resourceType string) {
	resType := *s.resourceTypes[subscriptionID][resourceType]

	for compositeKey, metricsByGrain := range resType.metricsByCompositeKey {
		now := time.Now().UTC()
		metricsByGrain.metricsValuesUpdated = now

		startTime := now.Add(time.Duration(-timeGrains[compositeKey.timeGrain]) * time.Second * 4) // times 4 because for some resources, data are missing for the very latest timestamp. The processing will keep only the latest timestamp with data.

		for region := range s.regions[subscriptionID] {
			clientMetrics, clientErr := s.GetMetricsBatchValuesClient(region)
			if clientErr != nil {
				s.settings.Logger.Error("failed to initialize the client to get Azure Metrics values", zap.Error(clientErr))
				return
			}

			start := 0
			for start < len(metricsByGrain.metrics) {
				end := start + s.cfg.MaximumNumberOfMetricsInACall
				if end > len(metricsByGrain.metrics) {
					end = len(metricsByGrain.metrics)
				}

				startResources := 0
				for startResources < len(resType.resourceIDs) {
					endResources := startResources + 50 // getBatch API is limited to 50 resources max
					if endResources > len(resType.resourceIDs) {
						endResources = len(resType.resourceIDs)
					}

					s.settings.Logger.Debug(
						"scrape",
						zap.String("subscription", subscriptionID),
						zap.String("region", region),
						zap.String("resourceType", resourceType),
						zap.Any("resourceIDs", resType.resourceIDs[startResources:endResources]),
						zap.Any("metrics", metricsByGrain.metrics[start:end]),
						zap.Int("startResources", startResources),
						zap.Int("endResources", endResources),
						zap.Time("startTime", startTime),
						zap.Time("endTime", now),
						zap.String("interval", compositeKey.timeGrain),
					)

					opts := newQueryResourcesOptions(
						compositeKey.dimensions,
						compositeKey.timeGrain,
						compositeKey.aggregations,
						startTime,
						now,
						s.cfg.MaximumNumberOfRecordsPerResource,
					)

					response, err := clientMetrics.QueryResources(
						ctx,
						subscriptionID,
						resourceType,
						metricsByGrain.metrics[start:end],
						azmetrics.ResourceIDList{ResourceIDs: resType.resourceIDs[startResources:endResources]},
						&opts,
					)
					if err != nil {
						var respErr *azcore.ResponseError
						if errors.As(err, &respErr) {
							s.settings.Logger.Error("failed to get Azure Metrics values data", zap.String("subscription", subscriptionID), zap.String("region", region), zap.String("resourceType", resourceType), zap.Any("metrics", metricsByGrain.metrics[start:end]), zap.Any("resources", resType.resourceIDs[startResources:endResources]), zap.Any("response", response), zap.Error(err))
						}
						s.settings.Logger.Error("failed to get Azure Metrics values data", zap.String("subscription", subscriptionID), zap.String("region", region), zap.String("resourceType", resourceType), zap.Any("metrics", metricsByGrain.metrics[start:end]), zap.Any("resources", resType.resourceIDs[startResources:endResources]), zap.Any("response", response), zap.Any("responseError", respErr))
						break
					}

					s.settings.Logger.Debug("response", zap.Any("raw", response))

					for _, metricValues := range response.Values {
						if metricValues.ResourceID == nil {
							continue
						}
						resID := *metricValues.ResourceID
						for _, metric := range metricValues.Values {
							for _, timeseriesElement := range metric.TimeSeries {
								res := s.resources[subscriptionID][resID]
								if res == nil {
									continue
								}
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
								attributes["timegrain"] = &compositeKey.timeGrain
								for i := len(timeseriesElement.Data) - 1; i >= 0; i-- { // reverse for loop because newest timestamp is at the end of the slice
									metricValue := timeseriesElement.Data[i]
									if metricValueIsNotEmpty(metricValue) {
										s.processQueryTimeseriesData(resID, metric, metricValue, attributes)
										break
									}
								}
							}
						}
					}
					startResources = endResources
				}
				start = end
			}
		}
	}
}

// newQueryResourcesOptions builds the options to make the QueryResources request.
func newQueryResourcesOptions(
	dimensionsStr string,
	timeGrain string,
	aggregationsStr string,
	start time.Time,
	end time.Time,
	top int32,
) azmetrics.QueryResourcesOptions {
	return azmetrics.QueryResourcesOptions{
		Aggregation: to.Ptr(aggregationsStr),
		StartTime:   to.Ptr(start.Format(time.RFC3339)),
		EndTime:     to.Ptr(end.Format(time.RFC3339)),
		Interval:    to.Ptr(timeGrain),
		Top:         to.Ptr(top), // Defaults to 10 (may be limiting results)
		Filter:      buildDimensionsFilter(dimensionsStr),
	}
}

// metricValueIsNotEmpty checks if the metric value is empty.
// This is necessary to compensate for the fact that Azure Monitor sometimes returns empty values.
func metricValueIsNotEmpty(metricValue azmetrics.MetricValue) bool {
	// Using an "or" chain is a bet on performance improvement. Assuming that it's not checking others if one is not nil. Not strictly verified though.
	return metricValue.Average != nil || metricValue.Count != nil || metricValue.Maximum != nil || metricValue.Minimum != nil || metricValue.Total != nil
}

func (s *azureBatchScraper) processQueryTimeseriesData(
	resourceID string,
	metric azmetrics.Metric,
	metricValue azmetrics.MetricValue,
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
