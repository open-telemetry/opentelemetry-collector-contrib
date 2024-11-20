// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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
	attributeLocation      = "location"
	attributeName          = "name"
	attributeResourceGroup = "resource_group"
	attributeResourceType  = "type"
	metadataPrefix         = "metadata_"
	tagPrefix              = "tags_"
)

// azureSubscription is an extract of armsubscriptions.Subscription.
// It designates a common structure between complex structures retrieved from the AP
// and simple subscriptions ids that you can find in config.
type azureSubscription struct {
	SubscriptionID   string
	DisplayName      *string
	resourcesUpdated time.Time
}

type azureResource struct {
	attributes                map[string]*string
	metricsByCompositeKey     map[metricsCompositeKey]*azureResourceMetrics
	metricsDefinitionsUpdated time.Time
	tags                      map[string]*string
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

func newScraper(conf *Config, settings receiver.Settings) *azureScraper {
	return &azureScraper{
		cfg:                      conf,
		settings:                 settings.TelemetrySettings,
		mb:                       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		azDefaultCredentialsFunc: azidentity.NewDefaultAzureCredential,
		azIDCredentialsFunc:      azidentity.NewClientSecretCredential,
		azIDWorkloadFunc:         azidentity.NewWorkloadIdentityCredential,
		azManagedIdentityFunc:    azidentity.NewManagedIdentityCredential,
		mutex:                    &sync.Mutex{},
	}
}

type azureScraper struct {
	cred azcore.TokenCredential
	
	cfg      *Config
	settings component.TelemetrySettings
	// resources on which we'll collect metrics. Stored by resource id and subscription id.
	resources map[string]map[string]*azureResource
	// subscriptions on which we'll look up resources. Stored by subscription id.
	subscriptions                 map[string]*azureSubscription
	subscriptionsUpdated          time.Time
	mb                            *metadata.MetricsBuilder
	azDefaultCredentialsFunc      func(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error)
	azIDCredentialsFunc           func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	azIDWorkloadFunc              func(options *azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error)
	azManagedIdentityFunc         func(options *azidentity.ManagedIdentityCredentialOptions) (*azidentity.ManagedIdentityCredential, error)
	armSubscriptionsClientOptions *arm.ClientOptions
	armResourcesClientOptions     *arm.ClientOptions
	armMonitorClientOptions       *arm.ClientOptions
	mutex                         *sync.Mutex
}

func (s *azureScraper) getArmClientOptions() *arm.ClientOptions {
	var cloudToUse cloud.Configuration
	switch s.cfg.Cloud {
	case azureGovernmentCloud:
		cloudToUse = cloud.AzureGovernment
	case azureChinaCloud:
		cloudToUse = cloud.AzureChina
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

func (s *azureScraper) start(_ context.Context, _ component.Host) (err error) {
	if err = s.loadCredentials(); err != nil {
		return err
	}

	// Init the client options for each azure API
	armClientOptions := s.getArmClientOptions()
	s.armSubscriptionsClientOptions = armClientOptions
	s.armResourcesClientOptions = armClientOptions
	s.armMonitorClientOptions = armClientOptions

	s.subscriptions = map[string]*azureSubscription{}
	s.resources = map[string]map[string]*azureResource{}

	// Initialize subscription ids from the config. Will be overridden if discovery is enabled anyway.
	ids := []string{s.cfg.SubscriptionID}
	ids = append(ids, s.cfg.SubscriptionIDs...)
	for _, id := range ids {
		if id != "" {
			s.resources[id] = make(map[string]*azureResource)
			s.subscriptions[id] = &azureSubscription{
				SubscriptionID: id,
			}
		}
	}

	return
}

func (s *azureScraper) loadCredentials() (err error) {
	switch s.cfg.Authentication {
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
		return fmt.Errorf("unknown authentication %v", s.cfg.Authentication)
	}
	return nil
}

func (s *azureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.getSubscriptions(ctx)

	var wg sync.WaitGroup
	for subscriptionID := range s.subscriptions {
		wg.Add(1)
		go func(subscriptionID string) {
			defer wg.Done()

			s.getResources(ctx, subscriptionID)

			resourcesIDsWithDefinitions := make(chan string)
			go func(subscriptionID string) {
				defer close(resourcesIDsWithDefinitions)
				for resourceID := range s.resources[subscriptionID] {
					s.getResourceMetricsDefinitions(ctx, subscriptionID, resourceID)
					resourcesIDsWithDefinitions <- resourceID
				}
			}(subscriptionID)

			var wg2 sync.WaitGroup
			for resourceID := range resourcesIDsWithDefinitions {
				wg2.Add(1)
				go func(subscriptionID, resourceID string) {
					defer wg2.Done()
					s.getResourceMetricsValues(ctx, subscriptionID, resourceID)
				}(subscriptionID, resourceID)
			}

			wg2.Wait()
		}(subscriptionID)

	}

	wg.Wait()

	return s.mb.Emit(
		metadata.WithAzureMonitorTenantID(s.cfg.TenantID),
	), nil
}

func (s *azureScraper) getSubscriptions(ctx context.Context) {
	if !s.cfg.DiscoverSubscriptions || !(time.Since(s.subscriptionsUpdated).Seconds() < s.cfg.CacheResources) {
		return
	}

	// if subscriptions discovery is enabled, we'll need a client
	armSubscriptionClient, clientErr := armsubscriptions.NewClient(s.cred, s.armSubscriptionsClientOptions)
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Subscriptions", zap.Error(clientErr))
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
			s.resources[*subscription.SubscriptionID] = make(map[string]*azureResource)
			s.subscriptions[*subscription.SubscriptionID] = &azureSubscription{
				SubscriptionID: *subscription.SubscriptionID,
				DisplayName:    subscription.DisplayName,
			}
			delete(existingSubscriptions, *subscription.SubscriptionID)
		}
	}
	if len(existingSubscriptions) > 0 {
		for idToDelete := range existingSubscriptions {
			delete(s.resources, idToDelete)
			delete(s.subscriptions, idToDelete)
		}
	}

	s.subscriptionsUpdated = time.Now()
	return
}

func (s *azureScraper) getResources(ctx context.Context, subscriptionID string) {
	if time.Since(s.subscriptions[subscriptionID].resourcesUpdated).Seconds() < s.cfg.CacheResources {
		return
	}
	clientResources, clientErr := armresources.NewClient(subscriptionID, s.cred, s.armResourcesClientOptions)
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Resources", zap.Error(clientErr))
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
					attributes[attributeLocation] = resource.Location
				}
				s.resources[subscriptionID][*resource.ID] = &azureResource{
					attributes: attributes,
					tags:       resource.Tags,
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
	return
}

func getResourceGroupFromID(id string) string {
	s := regexp.MustCompile(`\/resourcegroups/([^\/]+)\/`)
	match := s.FindStringSubmatch(strings.ToLower(id))

	if len(match) == 2 {
		return match[1]
	}
	return ""
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

func (s *azureScraper) getResourceMetricsDefinitions(ctx context.Context, subscriptionID, resourceID string) {
	if time.Since(s.resources[subscriptionID][resourceID].metricsDefinitionsUpdated).Seconds() < s.cfg.CacheResourcesDefinitions {
		return
	}

	clientMetricsDefinitions, clientErr := armmonitor.NewMetricDefinitionsClient(subscriptionID, s.cred, s.armMonitorClientOptions)
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Metrics definitions", zap.Error(clientErr))
		return
	}

	s.resources[subscriptionID][resourceID].metricsByCompositeKey = map[metricsCompositeKey]*azureResourceMetrics{}

	pager := clientMetricsDefinitions.NewListPager(resourceID, nil)
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
				compositeKey.dimensions = strings.Join(dimensionsSlice, ",")
			}
			s.storeMetricsDefinition(subscriptionID, resourceID, name, compositeKey)
		}
	}
	s.resources[subscriptionID][resourceID].metricsDefinitionsUpdated = time.Now()
}

func (s *azureScraper) storeMetricsDefinition(subscriptionID, resourceID, name string, compositeKey metricsCompositeKey) {
	if _, ok := s.resources[subscriptionID][resourceID].metricsByCompositeKey[compositeKey]; ok {
		s.resources[subscriptionID][resourceID].metricsByCompositeKey[compositeKey].metrics = append(
			s.resources[subscriptionID][resourceID].metricsByCompositeKey[compositeKey].metrics, name,
		)
	} else {
		s.resources[subscriptionID][resourceID].metricsByCompositeKey[compositeKey] = &azureResourceMetrics{metrics: []string{name}}
	}
}

func (s *azureScraper) getResourceMetricsValues(ctx context.Context, subscriptionID, resourceID string) {
	res := *s.resources[subscriptionID][resourceID]

	clientMetricsValues, clientErr := armmonitor.NewMetricsClient(subscriptionID, s.cred, s.armMonitorClientOptions)
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Metrics values", zap.Error(clientErr))
		return
	}

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
				s.cfg.MaximumNumberOfRecordsPerResource,
			)
			start = end

			result, err := clientMetricsValues.List(
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
						for name, value := range res.attributes {
							attributes[name] = value
						}
						for _, value := range timeseriesElement.Metadatavalues {
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
							s.processTimeseriesData(subscriptionID, resourceID, metric, metricValue, attributes)
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
	top int32,
) armmonitor.MetricsClientListOptions {
	resType := strings.Join(metrics[start:end], ",")
	filter := armmonitor.MetricsClientListOptions{
		Metricnames: &resType,
		Interval:    to.Ptr(timeGrain),
		Timespan:    to.Ptr(timeGrain),
		Aggregation: to.Ptr(strings.Join(aggregations, ",")),
		Top:         to.Ptr(top),
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
	subscriptionID, resourceID string,
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
				subscriptionID,
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
