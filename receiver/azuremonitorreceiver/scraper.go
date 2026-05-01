// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/huandu/go-clone"
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
	storageAccountType     = "Microsoft.Storage/storageAccounts"
)

// updatedMap is used internally to decorate each loaded data with a timestamp to do some cache.
type updatedMap[K comparable, V any] struct {
	LastUpdated time.Time
	Data        map[K]V
}

func newUpdatedMap[K comparable, V any]() *updatedMap[K, V] {
	return &updatedMap[K, V]{
		Data: map[K]V{},
	}
}

// azureSubscription is an extract of armsubscriptions.Subscription.
// It designates a common structure between complex structures retrieved from the AP
// and simple subscriptions ids that you can find in config.
type azureSubscription struct {
	SubscriptionID string
	DisplayName    string
}

// azureResource is an extract of armresources.GenericResourceExpanded.
// It contains only the necessary information for our use case:
//   - attributes are the labels that will appear in the resulting timeseries prefixed with metadataPrefix.
//     Directly collected from the Azure resource metadata like name, resource group, resource type, ...
//   - tags are the labels that will appear in the resulting timeseries prefixed with tagPrefix.
//     Directly collected from the Azure resource's tags.
//   - resourceType is a reference to the resource type, needed internally.
type azureResource struct {
	attributes   map[string]*string
	tags         map[string]*string
	resourceType *string
}

// metricsCompositeKey is a key used to uniquely identify a set of metrics.
// This is used to group the queries that will be done and typically to build the request option.
// It is composed of:
//   - dimensions: a sorted list of dimensions, used to group metrics.
//   - aggregations: a sorted list of aggregations, used to group metrics.
//   - timeGrain: the time grain of the metrics.
//
// Example:
//   - dimensions: "api_name,status_code"
//   - aggregations: "Average,Count"
//   - timeGrain: "PT1M"
type metricsCompositeKey struct {
	dimensions   string // comma separated sorted dimensions
	aggregations string // comma separated sorted aggregations
	timeGrain    string
}

// azureResourceMetrics is used in a map of metricsCompositeKey to store the metrics on which we'll query values for a given resource (armmonitor API) or resource type (AzBatch API)
//   - metrics: the list of metrics to query.
//   - metricsValuesUpdated: the last time the metrics values were updated.
//     This is used to avoid querying the same metrics multiple times.
//     It is updated when the metrics values are updated.
//     It is also used to avoid querying metrics that are not supported by the Azure API.
//     It is updated when the metrics definitions are updated.
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

type storageAccountSpecificConfig struct {
	askedBlobServices  bool
	askedFileServices  bool
	askedQueueServices bool
	askedTableServices bool
}

func newStorageAccountSpecificConfig(services []string) storageAccountSpecificConfig {
	return storageAccountSpecificConfig{
		askedBlobServices:  slices.IndexFunc(services, func(s string) bool { return strings.EqualFold(s, "Microsoft.Storage/storageAccounts/blobServices") }) != -1,
		askedFileServices:  slices.IndexFunc(services, func(s string) bool { return strings.EqualFold(s, "Microsoft.Storage/storageAccounts/fileServices") }) != -1,
		askedQueueServices: slices.IndexFunc(services, func(s string) bool { return strings.EqualFold(s, "Microsoft.Storage/storageAccounts/queueServices") }) != -1,
		askedTableServices: slices.IndexFunc(services, func(s string) bool { return strings.EqualFold(s, "Microsoft.Storage/storageAccounts/tableServices") }) != -1,
	}
}

func newScraper(conf *Config, settings receiver.Settings) *azureScraper {
	return &azureScraper{
		cfg:                          conf,
		settings:                     settings.TelemetrySettings,
		mb:                           metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		mutex:                        &sync.Mutex{},
		time:                         &timeWrapper{},
		clientOptionsResolver:        newClientOptionsResolver(conf.Cloud),
		storageAccountSpecificConfig: newStorageAccountSpecificConfig(conf.Services),
	}
}

// azSubscriptionStore is a convenient alias for azureScraper.subscriptions and azureBatchScraper.subscriptions fields
type azSubscriptionStore = *updatedMap[string, *azureSubscription]

// azResourceStore is a convenient alias for azureScraper.resources and azureBatchScraper.resources fields
type azResourceStore = map[string]*updatedMap[string, *azureResource]

// azMetricsStore is a convenient alias for azureScraper.metrics and azureBatchScraper.metrics fields
type azMetricsStore = map[string]map[string]*updatedMap[metricsCompositeKey, *azureResourceMetrics]

type azureScraper struct {
	cred     azcore.TokenCredential
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder

	// subscriptions on which we'll look up resources. Stored by subscription id.
	subscriptions azSubscriptionStore
	// resources on which we'll look up metrics. Stored by subscription id and resource id.
	resources azResourceStore
	// metrics on which we'll collect values. Stored by subscription id, resource id, and metricsCompositeKey.
	metrics azMetricsStore

	mutex                        *sync.Mutex
	time                         timeNowIface
	clientOptionsResolver        ClientOptionsResolver
	storageAccountSpecificConfig storageAccountSpecificConfig
}

func (s *azureScraper) start(_ context.Context, host component.Host) (err error) {
	if s.cred, err = loadCredentials(s.settings.Logger, s.cfg, host); err != nil {
		return err
	}

	s.subscriptions = newUpdatedMap[string, *azureSubscription]()
	s.resources = azResourceStore{}
	s.metrics = azMetricsStore{}

	return err
}

func (s *azureScraper) loadSubscription(sub azureSubscription) {
	s.subscriptions.Data[sub.SubscriptionID] = &azureSubscription{
		SubscriptionID: sub.SubscriptionID,
		DisplayName:    sub.DisplayName,
	}
	s.resources[sub.SubscriptionID] = newUpdatedMap[string, *azureResource]()
	s.metrics[sub.SubscriptionID] = make(map[string]*updatedMap[metricsCompositeKey, *azureResourceMetrics])
}

func (s *azureScraper) unloadSubscription(id string) {
	s.settings.Logger.Debug("Unloading subscription", zap.String("subscription_id", id))
	delete(s.subscriptions.Data, id)
	delete(s.resources, id)
	delete(s.metrics, id)
}

func (s *azureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.loadSubscriptions(ctx)

	for subscriptionID, subscription := range s.subscriptions.Data {
		s.loadResources(ctx, subscriptionID)

		resourcesIDsWithDefinitions := make(chan string)
		go func(subscriptionID string) {
			defer close(resourcesIDsWithDefinitions)
			for resourceID := range s.resources[subscriptionID].Data {
				s.loadMetricsDefinitions(ctx, subscriptionID, resourceID)
				resourcesIDsWithDefinitions <- resourceID
			}
		}(subscriptionID)

		var wg sync.WaitGroup
		for resourceID := range resourcesIDsWithDefinitions {
			wg.Add(1)
			go func(subscriptionID, resourceID string) {
				defer wg.Done()
				s.loadMetricsValues(ctx, subscriptionID, resourceID)
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

func (s *azureScraper) loadSubscriptions(ctx context.Context) {
	s.settings.Logger.Debug("Loading the list of Azure Subscriptions", zap.Bool("discover_subscriptions", s.cfg.DiscoverSubscriptions))
	if time.Since(s.subscriptions.LastUpdated).Seconds() < s.cfg.CacheResources {
		s.settings.Logger.Debug("Azure subscriptions are cached, skipping refresh")
		return
	}

	// Subscriptions discovery enabled or not, we'll need a client.
	// - If enabled, to get the subscription list
	// - If not, to get more info about the subscription
	// The only case where it won't be needed is when we don't want the subscription name in resource attributes.
	armSubscriptionClient, clientErr := armsubscriptions.NewClient(s.cred, s.clientOptionsResolver.GetArmSubscriptionsClientOptions())
	if clientErr != nil {
		s.settings.Logger.Error("Failed to initialize the client for Azure Subscriptions",
			zap.Error(clientErr))
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
			logFields := []zap.Field{zap.String("subscription_id", subID)}
			if err != nil {
				logFields = append(logFields, zap.Error(err))
				s.settings.Logger.Error("Failed to collect Subscription info from Azure", logFields...)
				return
			}
			logFields = append(logFields, zap.String("subscription_display_name", *resp.DisplayName))
			s.settings.Logger.Debug("Collected Subscription info from Azure", logFields...)
			s.loadSubscription(azureSubscription{
				SubscriptionID: *resp.SubscriptionID,
				DisplayName:    *resp.DisplayName,
			})
		}
		s.subscriptions.LastUpdated = time.Now()
		s.settings.Logger.Info("Loaded the list of Azure Subscriptions",
			zap.Int("subscriptions_count", len(s.subscriptions.Data)))
		return
	}

	// Prepare a map of existing subscriptions to detect removed ones later
	existingSubscriptions := map[string]void{}
	for id := range s.subscriptions.Data {
		existingSubscriptions[id] = void{}
	}

	opts := &armsubscriptions.ClientListOptions{}
	pager := armSubscriptionClient.NewListPager(opts)
	page := 0
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		logFields := []zap.Field{zap.Int("page", page)}
		if err != nil {
			logFields = append(logFields, zap.Error(err))
			s.settings.Logger.Error("Failed to collect Subscription list from Azure", logFields...)
			return
		}
		logFields = append(logFields, zap.Int("subscriptions_count", len(nextResult.Value)))
		s.settings.Logger.Debug("Collected Subscription list page from Azure", logFields...)
		page++

		for _, subscription := range nextResult.Value {
			s.loadSubscription(azureSubscription{
				SubscriptionID: *subscription.SubscriptionID,
				DisplayName:    *subscription.DisplayName,
			})
			delete(existingSubscriptions, *subscription.SubscriptionID)
		}
	}

	// Unload subscriptions that are no longer present
	if len(existingSubscriptions) > 0 {
		for idToDelete := range existingSubscriptions {
			s.unloadSubscription(idToDelete)
		}
	}

	s.subscriptions.LastUpdated = time.Now()
	s.settings.Logger.Info("Loaded the list of Azure Subscriptions",
		zap.Int("subscriptions_count", len(s.subscriptions.Data)),
		zap.Int("deleted_subscriptions_count", len(existingSubscriptions)))
}

func (s *azureScraper) loadResources(ctx context.Context, subscriptionID string) {
	s.settings.Logger.Debug("Loading the list of Azure Resources",
		zap.String("subscription_id", subscriptionID))

	// Ensure that the map for this subscription ID is initialized before trying to access it to avoid nil pointer dereference.
	if s.resources[subscriptionID] == nil {
		s.resources[subscriptionID] = newUpdatedMap[string, *azureResource]()
	}

	if time.Since(s.resources[subscriptionID].LastUpdated).Seconds() < s.cfg.CacheResources {
		s.settings.Logger.Debug("Azure Resources are cached, skipping refresh",
			zap.String("subscription_id", subscriptionID))
		return
	}

	clientResources, clientErr := armresources.NewClient(subscriptionID, s.cred, s.clientOptionsResolver.GetArmResourceClientOptions(subscriptionID))
	if clientErr != nil {
		s.settings.Logger.Error("Failed to initialize the client for Azure Resources",
			zap.String("subscription_id", subscriptionID),
			zap.Error(clientErr))
		return
	}

	// Prepare a map of existing resources to detect removed ones later
	existingResources := map[string]void{}
	for id := range s.resources[subscriptionID].Data {
		existingResources[id] = void{}
	}

	// Prepare the tags filter to apply on resources later on.
	// TODO: We don't need to do it per subscription. It can be done upper in the code to improve performances.
	tagsFilterMap := getTagsFilterMap(s.cfg.AppendTagsAsAttributes)

	// Prepare the options to get the resource list.
	filter := s.getResourcesFilter()
	opts := &armresources.ClientListOptions{
		Filter: &filter,
	}

	pager := clientResources.NewListPager(opts)
	page := 0
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)

		logFields := []zap.Field{
			zap.String("subscription_id", subscriptionID),
			zap.String("filter", filter),
			zap.Int("page", page),
		}
		if err != nil {
			logFields = append(logFields, zap.Error(err))
			s.settings.Logger.Error("Failed to collect Resource list from Azure", logFields...)
			return
		}
		logFields = append(logFields, zap.Int("resources_count", len(nextResult.Value)))
		s.settings.Logger.Debug("Collected Resource list from Azure", logFields...)
		page++

		for _, resource := range s.processResources(nextResult.Value) {
			if _, ok := s.resources[subscriptionID].Data[*resource.ID]; !ok {
				resourceGroup := getResourceGroupFromID(*resource.ID)
				attributes := map[string]*string{
					attributeName:          resource.Name,
					attributeResourceGroup: &resourceGroup,
					attributeResourceType:  resource.Type,
				}
				if resource.Location != nil {
					attributes[attributeLocation] = resource.Location
				}
				s.resources[subscriptionID].Data[*resource.ID] = &azureResource{
					attributes:   attributes,
					tags:         filterResourceTags(tagsFilterMap, resource.Tags),
					resourceType: resource.Type,
				}
			}
			delete(existingResources, *resource.ID)
		}
	}

	// Unload resources that are no longer present
	if len(existingResources) > 0 {
		for idToDelete := range existingResources {
			delete(s.resources[subscriptionID].Data, idToDelete)
		}
	}

	s.resources[subscriptionID].LastUpdated = time.Now()

	// Pre-allocate the per-resource metrics entries here, while we are still on the synchronous
	// path of scrape() (no goroutine has been launched yet for this subscription). The
	// producer (loadMetricsDefinitions) and the consumers (loadMetricsValues) will both access
	// s.metrics[subscriptionID]; Go maps are not safe for concurrent read/write even on different
	// keys (a write may trigger a rehash that invalidates concurrent reads). By inserting all
	// entries up front, we guarantee that only inner-struct fields are mutated concurrently
	// afterwards, never the outer map itself. Covered by TestAzureScraperBatchScrape_NoRaceWithManyResourceTypes
	// and the equivalent test on the non-batch scraper.
	if s.metrics[subscriptionID] == nil {
		s.metrics[subscriptionID] = make(map[string]*updatedMap[metricsCompositeKey, *azureResourceMetrics])
	}
	for resourceID := range s.resources[subscriptionID].Data {
		if s.metrics[subscriptionID][resourceID] == nil {
			s.metrics[subscriptionID][resourceID] = newUpdatedMap[metricsCompositeKey, *azureResourceMetrics]()
		}
	}

	s.settings.Logger.Info("Loaded the list of Azure Resources",
		zap.String("subscription_id", subscriptionID),
		zap.Int("resources_count", len(s.resources[subscriptionID].Data)),
		zap.Int("deleted_resources_count", len(existingResources)))
}

// processResources is a workaround specially done for the storageAccount metrics.
// Every StorageAccount resources have some implicit sub resources (/blobServices/default, fileServices/default, etc...) that are not returned by the API.
// We need to add them manually to the resources and resourceTypes map.
// Note that we do that hack only if the user has asked the sub resource types explicitly in the services config.
// Example:
// For each resource with id .../Microsoft.Storage/storageAccount/myResource of type Microsoft.Storage/storageAccount,
// It will create a fake resource with id .../Microsoft.Storage/storageAccount/myResource/blobServices/default of type Microsoft.Storage/storageAccounts/blobServices.
// TODO: duplicate
func (s *azureScraper) processResources(resources []*armresources.GenericResourceExpanded) []*armresources.GenericResourceExpanded {
	var subTypeResources []*armresources.GenericResourceExpanded
	for _, resource := range resources {
		subTypeResources = append(subTypeResources, resource)
		if resource != nil && resource.Type != nil && *resource.Type == storageAccountType {
			if s.storageAccountSpecificConfig.askedBlobServices {
				if r := buildSubTypeResource(*resource, "Microsoft.Storage/storageAccounts/blobServices", fmt.Sprintf("%s/blobServices/default", *resource.ID)); r != nil {
					subTypeResources = append(subTypeResources, r)
				}
			}
			if s.storageAccountSpecificConfig.askedFileServices {
				if r := buildSubTypeResource(*resource, "Microsoft.Storage/storageAccounts/fileServices", fmt.Sprintf("%s/fileServices/default", *resource.ID)); r != nil {
					subTypeResources = append(subTypeResources, r)
				}
			}
			if s.storageAccountSpecificConfig.askedQueueServices {
				if r := buildSubTypeResource(*resource, "Microsoft.Storage/storageAccounts/queueServices", fmt.Sprintf("%s/queueServices/default", *resource.ID)); r != nil {
					subTypeResources = append(subTypeResources, r)
				}
			}
			if s.storageAccountSpecificConfig.askedTableServices {
				if r := buildSubTypeResource(*resource, "Microsoft.Storage/storageAccounts/tableServices", fmt.Sprintf("%s/tableServices/default", *resource.ID)); r != nil {
					subTypeResources = append(subTypeResources, r)
				}
			}
		}
	}
	return subTypeResources
}

// buildSubTypeResource creates a virtual new resource with given type and ID.
// The rest of the attributes (location, tags, etc...) are copied from the original resource.
func buildSubTypeResource(orig armresources.GenericResourceExpanded, newType, newID string) *armresources.GenericResourceExpanded {
	cloned := clone.Clone(orig).(armresources.GenericResourceExpanded)
	cloned.ID = &newID
	cloned.Type = &newType
	return &cloned
}

func getResourceGroupFromID(id string) string {
	s := regexp.MustCompile(`/resourcegroups/([^/]+)/`)
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

func (s *azureScraper) loadMetricsDefinitions(ctx context.Context, subscriptionID, resourceID string) {
	s.settings.Logger.Debug("Loading the list of Azure Metrics Definitions",
		zap.String("resource_id", resourceID),
		zap.String("subscription_id", subscriptionID))

	if time.Since(s.metrics[subscriptionID][resourceID].LastUpdated).Seconds() < s.cfg.CacheResourcesDefinitions {
		s.settings.Logger.Debug("Azure Metrics Definitions are cached, skipping refresh",
			zap.String("resource_id", resourceID),
			zap.String("subscription_id", subscriptionID))
		return
	}

	// Prepare the map of metrics by composite key.
	s.metrics[subscriptionID][resourceID].Data = map[metricsCompositeKey]*azureResourceMetrics{}

	clientMetricsDefinitions, clientErr := armmonitor.NewMetricDefinitionsClient(subscriptionID, s.cred, s.clientOptionsResolver.GetArmMonitorClientOptions())
	if clientErr != nil {
		s.settings.Logger.Error("Failed to initialize the client for Azure Metrics definitions",
			zap.Error(clientErr))
		return
	}

	pager := clientMetricsDefinitions.NewListPager(resourceID, nil)

	page := 0
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)

		logFields := []zap.Field{
			zap.String("resource_id", resourceID),
			zap.String("subscription_id", subscriptionID),
			zap.Int("page", page),
		}
		if err != nil {
			logFields = append(logFields, zap.Error(err))
			s.settings.Logger.Error("Failed to collect Azure Definitions list from Azure", logFields...)
			return
		}
		logFields = append(logFields, zap.Int("definitions_count", len(nextResult.Value)))
		s.settings.Logger.Debug("Collected Azure Metrics Definitions list from Azure", logFields...)
		page++

		for _, v := range nextResult.Value {
			metricName := *v.Name.Value
			metricAggregations := getMetricAggregations(*v.Namespace, metricName, s.cfg.Metrics, convertAggregationsToStr(v.SupportedAggregationTypes))
			if len(metricAggregations) == 0 {
				continue
			}

			timeGrain := *v.MetricAvailabilities[0].TimeGrain
			dimensions := filterDimensions(v.Dimensions, s.cfg.Dimensions, *s.resources[subscriptionID].Data[resourceID].resourceType, metricName)
			compositeKey := metricsCompositeKey{
				timeGrain:    timeGrain,
				dimensions:   serializeDimensions(dimensions),
				aggregations: strings.Join(metricAggregations, ","),
			}
			s.loadMetricsDefinition(subscriptionID, resourceID, metricName, compositeKey)
		}
	}

	s.metrics[subscriptionID][resourceID].LastUpdated = time.Now()
	s.settings.Logger.Info("Loaded the list of Azure Metrics Definitions",
		zap.Int("metrics_definitions_count", len(s.metrics[subscriptionID][resourceID].Data)),
		zap.String("resource_id", resourceID),
		zap.String("subscription_id", subscriptionID))
}

func (s *azureScraper) loadMetricsDefinition(subscriptionID, resourceID, metricName string, compositeKey metricsCompositeKey) {
	s.settings.Logger.Debug("Loading metric definition",
		zap.String("dimensions", compositeKey.dimensions),
		zap.String("aggregations", compositeKey.aggregations),
		zap.String("timegrain", compositeKey.timeGrain),
		zap.String("metric", metricName),
		zap.String("resource_id", resourceID),
		zap.String("subscription_id", subscriptionID))
	if _, ok := s.metrics[subscriptionID][resourceID].Data[compositeKey]; ok {
		s.metrics[subscriptionID][resourceID].Data[compositeKey].metrics = append(
			s.metrics[subscriptionID][resourceID].Data[compositeKey].metrics, metricName,
		)
	} else {
		s.metrics[subscriptionID][resourceID].Data[compositeKey] = &azureResourceMetrics{metrics: []string{metricName}}
	}
}

func (s *azureScraper) loadMetricsValues(ctx context.Context, subscriptionID, resourceID string) {
	s.settings.Logger.Debug("Loading the Azure Metrics",
		zap.String("resource_id", resourceID),
		zap.String("subscription_id", subscriptionID))
	res := *s.resources[subscriptionID].Data[resourceID]
	metricsDef := s.metrics[subscriptionID][resourceID].Data
	updatedAt := s.time.Now().Truncate(truncateTimeGrain)

	clientMetricsValues, clientErr := armmonitor.NewMetricsClient(subscriptionID, s.cred, s.clientOptionsResolver.GetArmMonitorClientOptions())
	if clientErr != nil {
		s.settings.Logger.Error("Failed to initialize the client for Azure Metrics",
			zap.String("resource_id", resourceID),
			zap.String("subscription_id", subscriptionID),
			zap.Error(clientErr))
		return
	}

	for compositeKey, metricsByGrain := range metricsDef {
		if updatedAt.Sub(metricsByGrain.metricsValuesUpdated).Seconds() < float64(timeGrains[compositeKey.timeGrain]) {
			continue
		}
		metricsByGrain.metricsValuesUpdated = updatedAt

		start := 0

		for start < len(metricsByGrain.metrics) {
			end := min(start+s.cfg.MaximumNumberOfMetricsInACall, len(metricsByGrain.metrics))

			opts := newResourceMetricsValuesRequestOptions(
				metricsByGrain.metrics,
				compositeKey.dimensions,
				compositeKey.timeGrain,
				compositeKey.aggregations,
				start,
				end,
				s.cfg.MaximumNumberOfRecordsPerResource,
			)

			result, err := clientMetricsValues.List(
				ctx,
				resourceID,
				&opts,
			)
			logFields := []zap.Field{
				zap.Any("metrics", metricsByGrain.metrics[start:end]),
				zap.String("dimensions", compositeKey.dimensions),
				zap.String("aggregations", compositeKey.aggregations),
				zap.String("timegrain", compositeKey.timeGrain),
				zap.String("resource_id", resourceID),
				zap.String("subscription_id", subscriptionID),
			}
			if err != nil {
				logFields = append(logFields, zap.Error(err))
				s.settings.Logger.Error("Failed to collect Azure Metrics values from Azure", logFields...)
				return
			}

			start = end

			logFields = append(logFields, zap.Int("metrics_count", len(result.Value)))
			s.settings.Logger.Debug("Collected Azure Metrics values from Azure", logFields...)

			for _, metric := range result.Value {
				for _, timeseriesElement := range metric.Timeseries {
					if timeseriesElement.Data == nil {
						continue
					}
					attributes := map[string]*string{}
					maps.Copy(attributes, res.attributes)
					for _, value := range timeseriesElement.Metadatavalues {
						name := metadataPrefix + *value.Name.Value
						attributes[name] = value.Value
					}
					for tagName, value := range res.tags {
						name := tagPrefix + tagName
						attributes[name] = value
					}
					for _, metricValue := range timeseriesElement.Data {
						s.processTimeseriesData(resourceID, metric, metricValue, attributes)
					}
				}
			}
		}
	}
	s.settings.Logger.Info("Loaded the Azure Metrics",
		zap.String("resource_id", resourceID),
		zap.String("subscription_id", subscriptionID))
}

func newResourceMetricsValuesRequestOptions(
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

// getMetricAggregations returns a list of aggregations for a given namespace/metric.
// Two parameters are considered to know the aggregation to choose
// - a filter (given in configuration)
// - a list of supported aggregations (given by the API)
// If one namespace/metrics combination matches a provided filter,
// > Then it returns the aggregations in the filter
// > Otherwise it returns all supported aggregations.
// Note that a special filter * is supported to return all supported aggregations explicitly.
// /!\ It does not control the aggregations in the filters. If it's not in the supported list, it still lets it pass.
func getMetricAggregations(metricNamespace, metricName string, filters NestedListAlias, supportedAggregations []string) []string {
	// default behavior when no metric filters specified: pass all metrics with all aggregations
	if len(filters) == 0 {
		return supportedAggregations
	}

	metricsFilters, ok := mapFindInsensitive(filters, metricNamespace)
	// metric namespace isn't found, or it's empty: pass all metrics from the namespace
	if !ok || len(metricsFilters) == 0 {
		return supportedAggregations
	}

	aggregationsFilters, ok := mapFindInsensitive(metricsFilters, metricName)
	// if the target metric is absent in the metrics map: filter out metric
	if !ok {
		return []string{}
	}
	// allow all aggregations if others are not specified
	if len(aggregationsFilters) == 0 || slices.Contains(aggregationsFilters, filterAllAggregations) {
		return supportedAggregations
	}

	// collect known aggregations without filtering on supported
	var out []string
	for _, filter := range aggregationsFilters {
		for _, aggregation := range aggregations {
			if strings.EqualFold(aggregation, filter) {
				out = append(out, aggregation)
			}
		}
	}

	return out
}

func convertAggregationsToStr(aggregations []*armmonitor.AggregationType) []string {
	var result []string
	for _, aggr := range aggregations {
		result = append(result, string(*aggr))
	}
	return result
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

// getTagsFilterMap returns a map used to filter tags.
// Each user-configured tag key is normalized to lowercase and added to the map for case-insensitive lookup.
func getTagsFilterMap(appendTagsAsAttributes []string) (tagsFilterMap map[string]struct{}) {
	tagsFilterMap = make(map[string]struct{}, len(appendTagsAsAttributes))
	for _, v := range appendTagsAsAttributes {
		tagsFilterMap[strings.ToLower(v)] = struct{}{}
	}
	return tagsFilterMap
}

// filterResourceTags filter out resource tags according to configured tag list (append_tags_as_attributes)
func filterResourceTags(tagFilterList map[string]struct{}, resourceTags map[string]*string) map[string]*string {
	if _, includeAll := tagFilterList["*"]; includeAll {
		return resourceTags
	}

	// wildcard not found. include only configured tags
	includedTags := make(map[string]*string, len(resourceTags))
	for tagName, value := range resourceTags {
		if _, ok := tagFilterList[strings.ToLower(tagName)]; ok {
			includedTags[tagName] = value
		}
	}

	return includedTags
}
