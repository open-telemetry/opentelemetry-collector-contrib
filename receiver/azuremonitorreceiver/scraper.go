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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
    // "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

const (
	attributeLocation      = "location"
	attributeName          = "name"
	attributeResourceGroup = "resource_group"
	attributeResourceType  = "type"
	metadataPrefix         = "metadata_"
	tagPrefix              = "tags_"
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

// azureResource represents an Azure resource with its attributes, tags, and metrics definitions.
type azureResource struct {
	attributes                map[string]*string
	tags                      map[string]*string
	metricsByCompositeKey     map[metricsCompositeKey]*azureResourceMetrics
	metricsDefinitionsUpdated time.Time
}

// metricsCompositeKey uniquely identifies a set of metrics definitions based on time grain and dimensions.
type metricsCompositeKey struct {
	dimensions string // Comma-separated sorted dimensions
	timeGrain  string
}

// azureResourceMetrics holds the metrics names and the last time they were updated.
type azureResourceMetrics struct {
	metrics              []string
	metricsValuesUpdated time.Time
}

var resourceGroupRegex = regexp.MustCompile(`\/resourcegroups/([^\/]+)\/`)

func newScraper(conf *Config, settings receiver.Settings) *azureScraper {
	return &azureScraper{
		cfg:      conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		mutex:    &sync.Mutex{},
		// Credential functions for dependency injection
		azDefaultCredentialsFunc:        azidentity.NewDefaultAzureCredential,
		azIDCredentialsFunc:             azidentity.NewClientSecretCredential,
		azIDWorkloadFunc:                azidentity.NewWorkloadIdentityCredential,
		azManagedIdentityFunc:           azidentity.NewManagedIdentityCredential,
		// ARM client functions for dependency injection
		armClientFunc:                   armresources.NewClient,
		armMonitorDefinitionsClientFunc: armmonitor.NewMetricDefinitionsClient,
		armMonitorMetricsClientFunc:     armmonitor.NewMetricsClient,
	}
}

type azureScraper struct {
	cred                         azcore.TokenCredential
	clientResources              armClient
	clientMetricsDefinitions     metricsDefinitionsClientInterface
	clientMetricsValues          metricsValuesClient
	cfg                          *Config
	settings                     component.TelemetrySettings
	resources                    map[string]*azureResource
	resourcesUpdated             time.Time
	mb                           *metadata.MetricsBuilder
	armClientOptions             *arm.ClientOptions
	mutex                        *sync.Mutex
	azDefaultCredentialsFunc     func(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error)
	azIDCredentialsFunc          func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	azIDWorkloadFunc             func(options *azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error)
	azManagedIdentityFunc        func(options *azidentity.ManagedIdentityCredentialOptions) (*azidentity.ManagedIdentityCredential, error)
	armClientFunc                func(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error)
	armMonitorDefinitionsClientFunc func(string, azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error)
	armMonitorMetricsClientFunc     func(string, azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error)
}

type armClient interface {
	NewListPager(options *armresources.ClientListOptions) *runtime.Pager[armresources.ClientListResponse]
}

func (s *azureScraper) getArmClientOptions() *arm.ClientOptions {
	var cloudConfig cloud.Configuration
	switch s.cfg.Cloud {
	case azureGovernmentCloud:
		cloudConfig = cloud.AzureGovernment
	case azureChinaCloud:
		cloudConfig = cloud.AzureChina
	default:
		cloudConfig = cloud.AzurePublic
	}
	return &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cloudConfig,
		},
	}
}

func (s *azureScraper) getArmClient() (armClient, error) {
	return s.armClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
}

type metricsDefinitionsClientInterface interface {
	NewListPager(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) *runtime.Pager[armmonitor.MetricDefinitionsClientListResponse]
}

func (s *azureScraper) getMetricsDefinitionsClient() (metricsDefinitionsClientInterface, error) {
	return s.armMonitorDefinitionsClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
}

type metricsValuesClient interface {
	List(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error)
}

func (s *azureScraper) getMetricsValuesClient() (metricsValuesClient, error) {
	return s.armMonitorMetricsClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
}

func (s *azureScraper) start(_ context.Context, _ component.Host) error {
	if err := s.loadCredentials(); err != nil {
		return err
	}

	s.armClientOptions = s.getArmClientOptions()

	var err error
	s.clientResources, err = s.getArmClient()
	if err != nil {
		return err
	}

	s.clientMetricsDefinitions, err = s.getMetricsDefinitionsClient()
	if err != nil {
		return err
	}

	s.clientMetricsValues, err = s.getMetricsValuesClient()
	if err != nil {
		return err
	}

	s.resources = make(map[string]*azureResource)
	return nil
}

func (s *azureScraper) loadCredentials() error {
	var err error
	switch s.cfg.Authentication {
	case defaultCredentials:
		s.cred, err = s.azDefaultCredentialsFunc(nil)
	case servicePrincipal:
		s.cred, err = s.azIDCredentialsFunc(s.cfg.TenantID, s.cfg.ClientID, s.cfg.ClientSecret, nil)
	case workloadIdentity:
		s.cred, err = s.azIDWorkloadFunc(nil)
	case managedIdentity:
		var options *azidentity.ManagedIdentityCredentialOptions
		if s.cfg.ClientID != "" {
			options = &azidentity.ManagedIdentityCredentialOptions{
				ID: azidentity.ClientID(s.cfg.ClientID),
			}
		}
		s.cred, err = s.azManagedIdentityFunc(options)
	default:
		return fmt.Errorf("unknown authentication method: %v", s.cfg.Authentication)
	}
	return err
}

func (s *azureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.getResources(ctx)

	var wg sync.WaitGroup
	for resourceID := range s.resources {
		wg.Add(1)
		go func(resourceID string) {
			defer wg.Done()
			s.getResourceMetricsDefinitions(ctx, resourceID)
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

	existingResources := make(map[string]struct{})
	for id := range s.resources {
		existingResources[id] = struct{}{}
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
			resourceID := *resource.ID
			if _, ok := s.resources[resourceID]; !ok {
				resourceGroup := getResourceGroupFromID(resourceID)
				attributes := map[string]*string{
					attributeName:          resource.Name,
					attributeResourceGroup: &resourceGroup,
					attributeResourceType:  resource.Type,
				}
				if resource.Location != nil {
					attributes[attributeLocation] = resource.Location
				}
				s.resources[resourceID] = &azureResource{
					attributes: attributes,
					tags:       resource.Tags,
				}
			}
			delete(existingResources, resourceID)
		}
	}

	for idToDelete := range existingResources {
		delete(s.resources, idToDelete)
	}

	s.resourcesUpdated = time.Now()
}

func getResourceGroupFromID(id string) string {
	match := resourceGroupRegex.FindStringSubmatch(strings.ToLower(id))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func (s *azureScraper) getResourcesFilter() string {
	resourcesTypeFilter := strings.Join(s.cfg.Services, "' or resourceType eq '")

	var resourceGroupFilter string
	if len(s.cfg.ResourceGroups) > 0 {
		groups := strings.Join(s.cfg.ResourceGroups, "' or resourceGroup eq '")
		resourceGroupFilter = fmt.Sprintf(" and (resourceGroup eq '%s')", groups)
	}

	return fmt.Sprintf("(resourceType eq '%s')%s", resourcesTypeFilter, resourceGroupFilter)
}

func (s *azureScraper) getResourceMetricsDefinitions(ctx context.Context, resourceID string) {
	resource := s.resources[resourceID]
	if time.Since(resource.metricsDefinitionsUpdated).Seconds() < s.cfg.CacheResourcesDefinitions {
		return
	}

	resource.metricsByCompositeKey = make(map[metricsCompositeKey]*azureResourceMetrics)
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
				var dimensions []string
				for _, dimension := range v.Dimensions {
					value := strings.TrimSpace(*dimension.Value)
					if value != "" {
						dimensions = append(dimensions, value)
					}
				}
				sort.Strings(dimensions)
				compositeKey.dimensions = strings.Join(dimensions, ",")
			}
			s.storeMetricsDefinition(resourceID, name, compositeKey)
		}
	}
	resource.metricsDefinitionsUpdated = time.Now()
}

func (s *azureScraper) storeMetricsDefinition(resourceID, name string, compositeKey metricsCompositeKey) {
	resource := s.resources[resourceID]
	if metrics, ok := resource.metricsByCompositeKey[compositeKey]; ok {
		metrics.metrics = append(metrics.metrics, name)
	} else {
		resource.metricsByCompositeKey[compositeKey] = &azureResourceMetrics{metrics: []string{name}}
	}
}

func (s *azureScraper) getResourceMetricsValues(ctx context.Context, resourceID string) {
	resource := s.resources[resourceID]

	for compositeKey, metricsByGrain := range resource.metricsByCompositeKey {
		if time.Since(metricsByGrain.metricsValuesUpdated).Seconds() < float64(timeGrains[compositeKey.timeGrain]) {
			continue
		}
		metricsByGrain.metricsValuesUpdated = time.Now()

		metrics := metricsByGrain.metrics
		for start := 0; start < len(metrics); start += s.cfg.MaximumNumberOfMetricsInACall {
			end := start + s.cfg.MaximumNumberOfMetricsInACall
			if end > len(metrics) {
				end = len(metrics)
			}

			opts := getResourceMetricsValuesRequestOptions(
				metrics[start:end],
				compositeKey.dimensions,
				compositeKey.timeGrain,
				s.cfg.MaximumNumberOfRecordsPerResource,
			)

			result, err := s.clientMetricsValues.List(ctx, resourceID, &opts)
			if err != nil {
				s.settings.Logger.Error("failed to get Azure Metrics values data", zap.Error(err))
				return
			}

			for _, metric := range result.Value {
				for _, timeseriesElement := range metric.Timeseries {
					if timeseriesElement.Data != nil {
						attributes := copyAttributes(resource.attributes)
						for _, value := range timeseriesElement.Metadatavalues {
							attributes[metadataPrefix+*value.Name.Value] = value.Value
						}
						if s.cfg.AppendTagsAsAttributes {
							for tagName, value := range resource.tags {
								attributes[tagPrefix+tagName] = value
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
	top int32,
) armmonitor.MetricsClientListOptions {
	metricNames := strings.Join(metrics, ",")
	opts := armmonitor.MetricsClientListOptions{
		Metricnames: &metricNames,
		Interval:    to.Ptr(timeGrain),
		Timespan:    to.Ptr(timeGrain),
		Aggregation: to.Ptr(strings.Join(aggregations, ",")),
		Top:         to.Ptr(top),
	}

	if dimensionsStr != "" {
		var dimensionsFilter bytes.Buffer
		dimensions := strings.Split(dimensionsStr, ",")
		for i, dimension := range dimensions {
			dimensionsFilter.WriteString(fmt.Sprintf("%s eq '*'", dimension))
			if i < len(dimensions)-1 {
				dimensionsFilter.WriteString(" and ")
			}
		}
		filterString := dimensionsFilter.String()
		opts.Filter = &filterString
	}

	return opts
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

func copyAttributes(attrs map[string]*string) map[string]*string {
	newAttrs := make(map[string]*string, len(attrs))
	for k, v := range attrs {
		newAttrs[k] = v
	}
	return newAttrs
}
