// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"regexp"
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
	attributeLocation      = "location"
	attributeName          = "name"
	attributeResourceGroup = "resource_group"
	attributeResourceType  = "type"
	metadataPrefix         = "metadata_"
	tagPrefix              = "tags_"
	truncateTimeGrain      = time.Minute
)

type azureResource struct {
	attributes                map[string]*string
	metricsByCompositeKey     map[metricsCompositeKey]*azureResourceMetrics
	metricsDefinitionsUpdated time.Time
	tags                      map[string]*string
	resourceType              *string
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

type timeNowIface interface {
	Now() time.Time
}

type timeWrapper struct{}

func (*timeWrapper) Now() time.Time {
	return time.Now()
}

func newScraper(conf *Config, settings receiver.Settings) *azureScraper {
	return &azureScraper{
		cfg:                             conf,
		settings:                        settings.TelemetrySettings,
		mb:                              metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		azDefaultCredentialsFunc:        azidentity.NewDefaultAzureCredential,
		azIDCredentialsFunc:             azidentity.NewClientSecretCredential,
		azIDWorkloadFunc:                azidentity.NewWorkloadIdentityCredential,
		azManagedIdentityFunc:           azidentity.NewManagedIdentityCredential,
		armClientFunc:                   armresources.NewClient,
		armMonitorDefinitionsClientFunc: armmonitor.NewMetricDefinitionsClient,
		armMonitorMetricsClientFunc:     armmonitor.NewMetricsClient,
		mutex:                           &sync.Mutex{},
		time:                            &timeWrapper{},
	}
}

type azureScraper struct {
	cred azcore.TokenCredential

	clientResources          armClient
	clientMetricsDefinitions metricsDefinitionsClientInterface
	clientMetricsValues      metricsValuesClient

	cfg                             *Config
	settings                        component.TelemetrySettings
	resources                       map[string]*azureResource
	resourcesUpdated                time.Time
	mb                              *metadata.MetricsBuilder
	azDefaultCredentialsFunc        func(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error)
	azIDCredentialsFunc             func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	azIDWorkloadFunc                func(options *azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error)
	azManagedIdentityFunc           func(options *azidentity.ManagedIdentityCredentialOptions) (*azidentity.ManagedIdentityCredential, error)
	armClientOptions                *arm.ClientOptions
	armClientFunc                   func(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error)
	armMonitorDefinitionsClientFunc func(string, azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error)
	armMonitorMetricsClientFunc     func(string, azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error)
	mutex                           *sync.Mutex
	time                            timeNowIface
}

type armClient interface {
	NewListPager(options *armresources.ClientListOptions) *runtime.Pager[armresources.ClientListResponse]
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

func (s *azureScraper) getArmClient() (armClient, error) {
	client, err := s.armClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
	return client, err
}

type metricsDefinitionsClientInterface interface {
	NewListPager(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) *runtime.Pager[armmonitor.MetricDefinitionsClientListResponse]
}

func (s *azureScraper) getMetricsDefinitionsClient() (metricsDefinitionsClientInterface, error) {
	client, err := s.armMonitorDefinitionsClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
	return client, err
}

type metricsValuesClient interface {
	List(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (
		armmonitor.MetricsClientListResponse, error,
	)
}

func (s *azureScraper) GetMetricsValuesClient() (metricsValuesClient, error) {
	client, err := s.armMonitorMetricsClientFunc(s.cfg.SubscriptionID, s.cred, s.armClientOptions)
	return client, err
}

func (s *azureScraper) start(_ context.Context, _ component.Host) (err error) {
	if err = s.loadCredentials(); err != nil {
		return err
	}

	s.armClientOptions = s.getArmClientOptions()
	s.clientResources, err = s.getArmClient()
	if err != nil {
		return err
	}
	s.clientMetricsDefinitions, err = s.getMetricsDefinitionsClient()
	if err != nil {
		return err
	}
	s.clientMetricsValues, err = s.GetMetricsValuesClient()
	if err != nil {
		return err
	}

	s.resources = map[string]*azureResource{}

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
	s.getResources(ctx)
	resourcesIDsWithDefinitions := make(chan string)

	go func() {
		defer close(resourcesIDsWithDefinitions)
		for resourceID := range s.resources {
			s.getResourceMetricsDefinitions(ctx, resourceID)
			resourcesIDsWithDefinitions <- resourceID
		}
	}()

	var wg sync.WaitGroup
	for resourceID := range resourcesIDsWithDefinitions {
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
			delete(s.resources, idToDelete)
		}
	}

	s.resourcesUpdated = time.Now()
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
			metricName := *v.Name.Value
			dimensions := filterDimensions(v.Dimensions, s.cfg.Dimensions, *s.resources[resourceID].resourceType, metricName)
			compositeKey := metricsCompositeKey{
				timeGrain:  timeGrain,
				dimensions: serializeDimensions(dimensions),
			}
			s.storeMetricsDefinition(resourceID, metricName, compositeKey)
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
	updatedAt := s.time.Now().Truncate(truncateTimeGrain)

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
				start,
				end,
				s.cfg.MaximumNumberOfRecordsPerResource,
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
	start int,
	end int,
	top int32,
) armmonitor.MetricsClientListOptions {
	return armmonitor.MetricsClientListOptions{
		Metricnames: to.Ptr(strings.Join(metrics[start:end], ",")),
		Interval:    to.Ptr(timeGrain),
		Timespan:    to.Ptr(timeGrain),
		Aggregation: to.Ptr(strings.Join(aggregations, ",")),
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
