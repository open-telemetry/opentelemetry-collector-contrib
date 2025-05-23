// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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
	truncateTimeGrain      = time.Minute
	filterAllAggregations  = "*"
)

// azureSubscription is an extract of armsubscriptions.Subscription.
// It designates a common structure between complex structures retrieved from the AP
// and simple subscriptions ids that you can find in config.
type azureSubscription struct {
	SubscriptionID   string
	DisplayName      string
	resourcesUpdated time.Time
}

type azureResource struct {
	attributes                map[string]*string
	metricsByCompositeKey     map[metricsCompositeKey]*azureResourceMetrics
	metricsDefinitionsUpdated time.Time
	tags                      map[string]*string
	resourceType              *string
}

type metricsCompositeKey struct {
	dimensions   string // comma separated sorted dimensions
	aggregations string // comma separated sorted aggregations
	timeGrain    string
}

type azureResourceMetrics struct {
	metrics              []string
	metricsValuesUpdated time.Time
}

type void struct{}

type timeNowIface interface {
	Now() time.Time
}

type timeWrapper struct{}

func (*timeWrapper) Now() time.Time {
	return time.Now()
}

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
		time:                     &timeWrapper{},
		clientOptionsResolver:    newClientOptionsResolver(conf.Cloud),
	}
}

type azureScraper struct {
	cred azcore.TokenCredential

	cfg      *Config
	settings component.TelemetrySettings
	// resources on which we'll collect metrics. Stored by resource id and subscription id.
	resources map[string]map[string]*azureResource
	// subscriptions on which we'll look up resources. Stored by subscription id.
	subscriptions            map[string]*azureSubscription
	subscriptionsUpdated     time.Time
	mb                       *metadata.MetricsBuilder
	azDefaultCredentialsFunc func(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error)
	azIDCredentialsFunc      func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	azIDWorkloadFunc         func(options *azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error)
	azManagedIdentityFunc    func(options *azidentity.ManagedIdentityCredentialOptions) (*azidentity.ManagedIdentityCredential, error)

	mutex                 *sync.Mutex
	time                  timeNowIface
	clientOptionsResolver ClientOptionsResolver
}

func (s *azureScraper) start(_ context.Context, host component.Host) (err error) {
	if err = s.loadCredentials(host); err != nil {
		return err
	}

	s.subscriptions = map[string]*azureSubscription{}
	s.resources = map[string]map[string]*azureResource{}

	return
}

func (s *azureScraper) loadSubscription(sub azureSubscription) {
	s.resources[sub.SubscriptionID] = make(map[string]*azureResource)
	s.subscriptions[sub.SubscriptionID] = &azureSubscription{
		SubscriptionID: sub.SubscriptionID,
		DisplayName:    sub.DisplayName,
	}
}

func (s *azureScraper) unloadSubscription(id string) {
	delete(s.resources, id)
	delete(s.subscriptions, id)
}

func loadTokenProvider(host component.Host, idAuth component.ID) (azcore.TokenCredential, error) {
	authExtension, ok := host.GetExtensions()[idAuth]
	if !ok {
		return nil, fmt.Errorf("unknown azureauth extension %q", idAuth.String())
	}
	credential, ok := authExtension.(azcore.TokenCredential)
	if !ok {
		return nil, fmt.Errorf("extension %q does not implement azcore.TokenCredential", idAuth.String())
	}
	return credential, nil
}

func (s *azureScraper) loadCredentials(host component.Host) (err error) {
	if s.cfg.Authentication != nil {
		s.settings.Logger.Info("'auth.authenticator' will be used to get the token credential")
		if s.cred, err = loadTokenProvider(host, s.cfg.Authentication.AuthenticatorID); err != nil {
			return err
		}
		return nil
	}

	s.settings.Logger.Warn("'credentials' is deprecated, use 'auth.authenticator' instead")
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

func (s *azureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.getSubscriptions(ctx)

	for subscriptionID, subscription := range s.subscriptions {
		s.getResources(ctx, subscriptionID)

		resourcesIDsWithDefinitions := make(chan string)
		go func(subscriptionID string) {
			defer close(resourcesIDsWithDefinitions)
			for resourceID := range s.resources[subscriptionID] {
				s.getResourceMetricsDefinitions(ctx, subscriptionID, resourceID)
				resourcesIDsWithDefinitions <- resourceID
			}
		}(subscriptionID)

		var wg sync.WaitGroup
		for resourceID := range resourcesIDsWithDefinitions {
			wg.Add(1)
			go func(subscriptionID, resourceID string) {
				defer wg.Done()
				s.getResourceMetricsValues(ctx, subscriptionID, resourceID)
			}(subscriptionID, resourceID)
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

func (s *azureScraper) getSubscriptions(ctx context.Context) {
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

func (s *azureScraper) getResources(ctx context.Context, subscriptionID string) {
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
					attributes:   attributes,
					tags:         resource.Tags,
					resourceType: resource.Type,
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

	clientMetricsDefinitions, clientErr := armmonitor.NewMetricDefinitionsClient(subscriptionID, s.cred, s.clientOptionsResolver.GetArmMonitorClientOptions())
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
			metricName := *v.Name.Value
			metricAggregations := getMetricAggregations(*v.Namespace, metricName, s.cfg.Metrics)
			if len(metricAggregations) == 0 {
				continue
			}

			timeGrain := *v.MetricAvailabilities[0].TimeGrain
			dimensions := filterDimensions(v.Dimensions, s.cfg.Dimensions, *s.resources[subscriptionID][resourceID].resourceType, metricName)
			compositeKey := metricsCompositeKey{
				timeGrain:    timeGrain,
				dimensions:   serializeDimensions(dimensions),
				aggregations: strings.Join(metricAggregations, ","),
			}
			s.storeMetricsDefinition(subscriptionID, resourceID, metricName, compositeKey)
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
	updatedAt := s.time.Now().Truncate(truncateTimeGrain)

	clientMetricsValues, clientErr := armmonitor.NewMetricsClient(subscriptionID, s.cred, s.clientOptionsResolver.GetArmMonitorClientOptions())
	if clientErr != nil {
		s.settings.Logger.Error("failed to initialize the client to get Azure Metrics values", zap.Error(clientErr))
		return
	}

	for compositeKey, metricsByGrain := range res.metricsByCompositeKey {
		if updatedAt.Sub(metricsByGrain.metricsValuesUpdated).Seconds() < float64(timeGrains[compositeKey.timeGrain]) {
			continue
		}
		metricsByGrain.metricsValuesUpdated = updatedAt

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
				compositeKey.aggregations,
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
	aggregationsStr string,
	start int,
	end int,
	top int32,
) armmonitor.MetricsClientListOptions {
	return armmonitor.MetricsClientListOptions{
		Metricnames: to.Ptr(strings.Join(metrics[start:end], ",")),
		Interval:    to.Ptr(timeGrain),
		Timespan:    to.Ptr(timeGrain),
		Aggregation: to.Ptr(aggregationsStr),
		Top:         to.Ptr(top),
		Filter:      buildDimensionsFilter(dimensionsStr),
	}
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

func getMetricAggregations(metricNamespace, metricName string, filters NestedListAlias) []string {
	// default behavior when no metric filters specified: pass all metrics with all aggregations
	if len(filters) == 0 {
		return aggregations
	}

	metricsFilters, ok := mapFindInsensitive(filters, metricNamespace)
	// metric namespace not found or it's empty: pass all metrics from the namespace
	if !ok || len(metricsFilters) == 0 {
		return aggregations
	}

	aggregationsFilters, ok := mapFindInsensitive(metricsFilters, metricName)
	// if target metric is absent in metrics map: filter out metric
	if !ok {
		return []string{}
	}
	// allow all aggregations if others are not specified
	if len(aggregationsFilters) == 0 || slices.Contains(aggregationsFilters, filterAllAggregations) {
		return aggregations
	}

	// collect known supported aggregations
	out := []string{}
	for _, filter := range aggregationsFilters {
		for _, aggregation := range aggregations {
			if strings.EqualFold(aggregation, filter) {
				out = append(out, aggregation)
			}
		}
	}

	return out
}

func mapFindInsensitive[T any](m map[string]T, key string) (T, bool) {
	for k, v := range m {
		if strings.EqualFold(key, k) {
			return v, true
		}
	}

	var got T
	return got, false
}
