// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorreceiver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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

type azureResource struct {
	metricsByGrains           map[string]*azureResourceMetrics
	metricsDefinitionsUpdated int64
}

type azureResourceMetrics struct {
	metrics              []string
	metricsValuesUpdated int64
}

type void struct{}

func newScraper(conf *Config, settings receiver.CreateSettings) *azureScraper {
	return &azureScraper{
		cfg:                             conf,
		settings:                        settings.TelemetrySettings,
		mb:                              metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		azIdCredentialsFunc:             azidentity.NewClientSecretCredential,
		armClientFunc:                   armresources.NewClient,
		armMonitorDefinitionsClientFunc: armmonitor.NewMetricDefinitionsClient,
		armMonitorMetricsClientFunc:     armmonitor.NewMetricsClient,
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
	resourcesUpdated                int64
	mb                              *metadata.MetricsBuilder
	azIdCredentialsFunc             func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	armClientFunc                   func(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error)
	armMonitorDefinitionsClientFunc func(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error)
	armMonitorMetricsClientFunc     func(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error)
}

type ArmClient interface {
	NewListPager(options *armresources.ClientListOptions) *runtime.Pager[armresources.ClientListResponse]
}

func (s *azureScraper) getArmClient() ArmClient {
	client, _ := s.armClientFunc(s.cfg.SubscriptionId, s.cred, nil)
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
	List(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error)
}

func (s *azureScraper) GetMetricsValuesClient() MetricsValuesClient {
	client, _ := s.armMonitorMetricsClientFunc(s.cred, nil)
	return client
}

func (s *azureScraper) start(ctx context.Context, host component.Host) (err error) {
	s.cred, err = s.azIdCredentialsFunc(s.cfg.TenantId, s.cfg.ClientId, s.cfg.ClientSecret, nil)
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
		for resourceId := range s.resources {
			s.getResourceMetricsDefinitions(ctx, resourceId)
			resourcesIdsWithDefinitions <- resourceId
		}
	}()

	var resourceMetricsProgress sync.WaitGroup

	for resourcesIdsWithDefinitions != nil {
		select {
		case resourceId, ok := <-resourcesIdsWithDefinitions:
			if !ok {
				resourcesIdsWithDefinitions = nil
				break
			}
			resourceMetricsProgress.Add(1)
			go func() {
				defer resourceMetricsProgress.Done()
				s.getResourceMetricsValues(ctx, resourceId)
			}()
		}
	}

	resourceMetricsProgress.Wait()

	return s.mb.Emit(
		metadata.WithAzureMonitorSubscriptionID(s.cfg.SubscriptionId),
		metadata.WithAzureMonitorTenantID(s.cfg.TenantId),
	), nil
}

func (s *azureScraper) getResources(ctx context.Context) {
	if time.Now().UTC().Unix() < (s.resourcesUpdated + s.cfg.CacheResources) {
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
				s.resources[*resource.ID] = &azureResource{}
			}
			delete(existingResources, *resource.ID)
		}
	}
	if len(existingResources) > 0 {
		for idToDelete := range existingResources {
			delete(s.resources, idToDelete)
		}
	}

	s.resourcesUpdated = time.Now().UTC().Unix()
}

func (s *azureScraper) getResourcesFilter() string {
	// TODO: switch to parsing services from https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported
	var resourcesTypeFilter string
	if len(s.cfg.Services) > 0 {
		resourcesTypeFilter = strings.Join(s.cfg.Services, "' or resourceType eq '")
	} else {
		resourcesTypeFilter = strings.Join(monitorServices, "' or resourceType eq '")
	}

	resourcesGroupFilterString := ""
	if len(s.cfg.ResourceGroups) > 0 {
		resourcesGroupFilterString = fmt.Sprintf(" and (resourceGroup eq '%s')",
			strings.Join(s.cfg.ResourceGroups, "' or resourceGroup eq  '"))
	}

	return fmt.Sprintf("(resourceType eq '%s')%s", resourcesTypeFilter, resourcesGroupFilterString)
}

func (s *azureScraper) getResourceMetricsDefinitions(ctx context.Context, resourceId string) {

	if time.Now().UTC().Unix() < (s.resources[resourceId].metricsDefinitionsUpdated + s.cfg.CacheResourcesDefinitions) {
		return
	}
	res := s.resources[resourceId]
	res.metricsByGrains = map[string]*azureResourceMetrics{}

	pager := s.clientMetricsDefinitions.NewListPager(resourceId, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			s.settings.Logger.Error("failed to get Azure Metrics definitions data", zap.Error(err))
			return
		}
		for _, v := range nextResult.Value {

			timeGrain := *v.MetricAvailabilities[0].TimeGrain
			name := *v.Name.Value

			if _, ok := res.metricsByGrains[timeGrain]; ok {
				res.metricsByGrains[timeGrain].metrics = append(res.metricsByGrains[timeGrain].metrics, name)
			} else {
				res.metricsByGrains[timeGrain] = &azureResourceMetrics{metrics: []string{name}}
			}
		}
	}
	res.metricsDefinitionsUpdated = time.Now().UTC().Unix()
}

func (s *azureScraper) getResourceMetricsValues(ctx context.Context, resourceId string) {

	res := s.resources[resourceId]

	for timeGrain, metricsByGrain := range res.metricsByGrains {

		if time.Now().UTC().Unix() < (metricsByGrain.metricsValuesUpdated + timeGrains[timeGrain]) {
			continue
		}
		metricsByGrain.metricsValuesUpdated = time.Now().UTC().Unix()

		start, max := 0, s.cfg.MaximumNumberOfMetricsInACall

		for start < len(metricsByGrain.metrics) {

			end := start + max
			if end > len(metricsByGrain.metrics) {
				end = len(metricsByGrain.metrics)
			}

			resType := strings.Join(metricsByGrain.metrics[start:end], ",")
			start = end

			opts := armmonitor.MetricsClientListOptions{
				Metricnames: &resType,
				Interval:    to.Ptr(timeGrain),
				Timespan:    to.Ptr(timeGrain),
				Aggregation: to.Ptr(strings.Join(aggregations, ",")),
			}

			result, err := s.clientMetricsValues.List(
				ctx,
				resourceId,
				&opts,
			)
			if err != nil {
				s.settings.Logger.Error("failed to get Azure Metrics values data", zap.Error(err))
				return
			}

			for _, metric := range result.Value {

				for _, timeserie := range metric.Timeseries {
					if timeserie.Data != nil {
						for _, timeserieData := range timeserie.Data {

							ts := pcommon.NewTimestampFromTime(time.Now())
							if timeserieData.Average != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Average", string(*metric.Unit), ts, *timeserieData.Average)
							}
							if timeserieData.Count != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Count", string(*metric.Unit), ts, *timeserieData.Count)
							}
							if timeserieData.Maximum != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Maximum", string(*metric.Unit), ts, *timeserieData.Maximum)
							}
							if timeserieData.Minimum != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Minimum", string(*metric.Unit), ts, *timeserieData.Minimum)
							}
							if timeserieData.Total != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Total", string(*metric.Unit), ts, *timeserieData.Total)
							}
						}
					}
				}
			}
		}
	}
}
