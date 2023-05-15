// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

var (
	// Predefined error responses for configuration validation failures
	errMissingTenantID       = errors.New(`TenantID" is not specified in config`)
	errMissingSubscriptionID = errors.New(`SubscriptionID" is not specified in config`)
	errMissingClientID       = errors.New(`ClientID" is not specified in config`)
	errMissingClientSecret   = errors.New(`ClientSecret" is not specified in config`)

	monitorServices = []string{
		"Microsoft.EventGrid/eventSubscriptions",
		"Microsoft.EventGrid/topics",
		"Microsoft.EventGrid/domains",
		"Microsoft.EventGrid/extensionTopics",
		"Microsoft.EventGrid/systemTopics",
		"Microsoft.EventGrid/partnerNamespaces",
		"Microsoft.EventGrid/partnerTopics",
		"Microsoft.Logic/workflows",
		"Microsoft.Logic/integrationServiceEnvironments",
		"Microsoft.SignalRService/SignalR",
		"Microsoft.SignalRService/WebPubSub",
		"Microsoft.Batch/batchAccounts",
		"microsoft.insights/components",
		"microsoft.insights/autoscalesettings",
		"Microsoft.Automation/automationAccounts",
		"Microsoft.ContainerInstance/containerGroups",
		"Microsoft.Devices/IotHubs",
		"Microsoft.Devices/ProvisioningServices",
		"Microsoft.StorageSync/storageSyncServices",
		"Microsoft.DBforPostgreSQL/servers",
		"Microsoft.DBforPostgreSQL/flexibleServers",
		"Microsoft.DataShare/accounts",
		"Microsoft.AVS/privateClouds",
		"Microsoft.DataCollaboration/workspaces",
		"Microsoft.PowerBIDedicated/capacities",
		"Microsoft.ContainerService/managedClusters",
		"Microsoft.Sql/servers",
		"Microsoft.Sql/servers/databases",
		"Microsoft.Sql/servers/elasticpools",
		"Microsoft.Sql/managedInstances",
		"Microsoft.AnalysisServices/servers",
		"Microsoft.StreamAnalytics/streamingjobs",
		"microsoft.aadiam/azureADMetrics",
		"Microsoft.Cache/Redis",
		"Microsoft.Cache/redisEnterprise",
		"Microsoft.AppPlatform/Spring",
		"Microsoft.ContainerRegistry/registries",
		"Microsoft.EventHub/namespaces",
		"Microsoft.EventHub/clusters",
		"Microsoft.NetApp/netAppAccounts/capacityPools",
		"Microsoft.NetApp/netAppAccounts/capacityPools/volumes",
		"Microsoft.ClassicCompute/domainNames/slots/roles",
		"Microsoft.ClassicCompute/virtualMachines",
		"Microsoft.Compute/virtualMachines",
		"Microsoft.Compute/virtualMachineScaleSets",
		"Microsoft.Compute/virtualMachineScaleSets/virtualMachines",
		"Microsoft.Compute/cloudServices",
		"Microsoft.Compute/cloudServices/roles",
		"Microsoft.Peering/peerings",
		"Microsoft.Peering/peeringServices",
		"Microsoft.NotificationHubs/namespaces/notificationHubs",
		"Microsoft.AppConfiguration/configurationStores",
		"Microsoft.TimeSeriesInsights/environments",
		"Microsoft.TimeSeriesInsights/environments/eventsources",
		"Microsoft.ClassicStorage/storageAccounts",
		"Microsoft.ClassicStorage/storageAccounts/blobServices",
		"Microsoft.ClassicStorage/storageAccounts/tableServices",
		"Microsoft.ClassicStorage/storageAccounts/fileServices",
		"Microsoft.ClassicStorage/storageAccounts/queueServices",
		"Microsoft.Kusto/clusters",
		"Microsoft.MachineLearningServices/workspaces",
		"Microsoft.DBforMariaDB/servers",
		"Microsoft.Relay/namespaces",
		"Microsoft.OperationalInsights/workspaces",
		"Microsoft.Network/virtualNetworks",
		"Microsoft.Network/natGateways",
		"Microsoft.Network/publicIPAddresses",
		"Microsoft.Network/networkInterfaces",
		"Microsoft.Network/privateEndpoints",
		"Microsoft.Network/loadBalancers",
		"Microsoft.Network/networkWatchers/connectionMonitors",
		"Microsoft.Network/virtualNetworkGateways",
		"Microsoft.Network/connections",
		"Microsoft.Network/applicationGateways",
		"Microsoft.Network/dnszones",
		"Microsoft.Network/privateDnsZones",
		"Microsoft.Network/trafficmanagerprofiles",
		"Microsoft.Network/expressRouteCircuits",
		"Microsoft.Network/vpnGateways",
		"Microsoft.Network/p2sVpnGateways",
		"Microsoft.Network/expressRouteGateways",
		"Microsoft.Network/expressRoutePorts",
		"Microsoft.Network/azureFirewalls",
		"Microsoft.Network/privateLinkServices",
		"Microsoft.Network/frontdoors",
		"Microsoft.Network/virtualRouters",
		"Microsoft.Purview/accounts",
		"Microsoft.CognitiveServices/accounts",
		"Microsoft.Maps/accounts",
		"Microsoft.MixedReality/spatialAnchorsAccounts",
		"Microsoft.MixedReality/remoteRenderingAccounts",
		"Microsoft.DBforMySQL/servers",
		"Microsoft.DBforMySQL/flexibleServers",
		"Microsoft.DataBoxEdge/DataBoxEdgeDevices",
		"Microsoft.IoTCentral/IoTApps",
		"Microsoft.Web/staticSites",
		"Microsoft.Web/serverFarms",
		"Microsoft.Web/sites",
		"Microsoft.Web/sites/slots",
		"Microsoft.Web/hostingEnvironments",
		"Microsoft.Web/hostingEnvironments/multiRolePools",
		"Microsoft.Web/hostingEnvironments/workerPools",
		"Microsoft.Web/connections",
		"Microsoft.HDInsight/clusters",
		"Microsoft.Search/searchServices",
		"Microsoft.ServiceFabricMesh/applications",
		"Microsoft.HealthcareApis/services",
		"Microsoft.HealthcareApis/workspaces/analyticsconnectors",
		"Microsoft.HealthcareApis/workspaces/iotconnectors",
		"Microsoft.HealthcareApis/workspaces/fhirservices",
		"Microsoft.ApiManagement/service",
		"Microsoft.DataLakeAnalytics/accounts",
		"Microsoft.DocumentDB/databaseAccounts",
		"Microsoft.Synapse/workspaces",
		"Microsoft.Synapse/workspaces/bigDataPools",
		"Microsoft.Synapse/workspaces/sqlPools",
		"Microsoft.Synapse/workspaces/kustoPools",
		"Microsoft.Storage/storageAccounts",
		"Microsoft.Storage/storageAccounts/blobServices",
		"Microsoft.Storage/storageAccounts/tableServices",
		"Microsoft.Storage/storageAccounts/queueServices",
		"Microsoft.Storage/storageAccounts/fileServices",
		"Microsoft.Cdn/profiles",
		"Microsoft.Cdn/CdnWebApplicationFirewallPolicies",
		"Microsoft.KeyVault/vaults",
		"Microsoft.KeyVault/managedHSMs",
		"Microsoft.Media/mediaservices",
		"Microsoft.Media/mediaservices/streamingEndpoints",
		"Microsoft.Media/mediaservices/liveEvents",
		"Microsoft.DigitalTwins/digitalTwinsInstances",
		"Microsoft.DataFactory/dataFactories",
		"Microsoft.DataFactory/factories",
		"Microsoft.DataLakeStore/accounts",
		"Microsoft.ServiceBus/namespaces",
		"Microsoft.AAD/DomainServices",
		"Microsoft.Orbital/spacecrafts",
		"Microsoft.Orbital/contactProfiles",
		"Microsoft.Logic/IntegrationServiceEnvironments",
		"Microsoft.Logic/Workflows",
		"Microsoft.Batch/batchaccounts",
		"Microsoft.HybridContainerService/provisionedClusters",
		"microsoft.securitydetonation/chambers",
		"microsoft.hybridnetwork/networkfunctions",
		"Microsoft.DBForPostgreSQL/serverGroupsv2",
		"microsoft.avs/privateClouds",
		"Microsoft.StorageMover/storageMovers",
		"Microsoft.NetworkFunction/azureTrafficCollectors",
		"Microsoft.Cache/redis",
		"Microsoft.DataProtection/BackupVaults",
		"Microsoft.ConnectedVehicle/platformAccounts",
		"Microsoft.EventHub/Namespaces",
		"Microsoft.ConnectedCache/ispCustomers",
		"Microsoft.ConnectedCache/CacheNodes",
		"Microsoft.Compute/cloudservices",
		"microsoft.compute/disks",
		"Microsoft.Compute/virtualmachineScaleSets",
		"Wandisco.Fusion/migrators/liveDataMigrations",
		"Wandisco.Fusion/migrators",
		"Wandisco.Fusion/migrators/metadataMigrations",
		"Microsoft.VoiceServices/CommunicationsGateways",
		"microsoft.kubernetes/connectedClusters",
		"Microsoft.PlayFab/titles",
		"Microsoft.Communication/CommunicationServices",
		"Microsoft.ManagedNetworkFabric/networkDevices",
		"Microsoft.MachineLearningServices/workspaces/onlineEndpoints",
		"Microsoft.MachineLearningServices/workspaces/onlineEndpoints/deployments",
		"Microsoft.RecoveryServices/Vaults",
		"Microsoft.StorageCache/caches",
		"Microsoft.StorageCache/amlFilesystems",
		"microsoft.purview/accounts",
		"Microsoft.Network/applicationgateways",
		"Microsoft.Network/dnsForwardingRulesets",
		"microsoft.network/virtualnetworkgateways",
		"microsoft.network/vpngateways",
		"Microsoft.Network/dnsResolvers",
		"microsoft.network/bastionHosts",
		"microsoft.network/expressroutegateways",
		"microsoft.network/p2svpngateways",
		"Microsoft.Network/virtualHubs",
		"Microsoft.Web/containerapps",
		"Microsoft.Web/serverfarms",
		"Microsoft.Web/hostingenvironments/multirolepools",
		"Microsoft.Web/hostingenvironments/workerpools",
		"microsoft.singularity/accounts",
		"Microsoft.DocumentDB/DatabaseAccounts",
		"Microsoft.DocumentDB/cassandraClusters",
		"Microsoft.Storage/storageAccounts/objectReplicationPolicies",
		"Microsoft.Cdn/cdnwebapplicationfirewallpolicies",
		"microsoft.keyvault/managedhsms",
		"Microsoft.Dashboard/grafana",
		"Microsoft.Cloudtest/hostedpools",
		"Microsoft.Cloudtest/pools",
		"Microsoft.Monitor/accounts",
		"Microsoft.App/containerapps",
		"Microsoft.App/managedEnvironments",
		"Microsoft.ServiceBus/Namespaces",
	}
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	MetricsBuilderConfig                    metadata.MetricsBuilderConfig `mapstructure:",squash"`
	SubscriptionID                          string                        `mapstructure:"subscription_id"`
	TenantID                                string                        `mapstructure:"tenant_id"`
	ClientID                                string                        `mapstructure:"client_id"`
	ClientSecret                            string                        `mapstructure:"client_secret"`
	ResourceGroups                          []string                      `mapstructure:"resource_groups"`
	Services                                []string                      `mapstructure:"services"`
	CacheResources                          float64                       `mapstructure:"cache_resources"`
	CacheResourcesDefinitions               float64                       `mapstructure:"cache_resources_definitions"`
	MaximumNumberOfMetricsInACall           int                           `mapstructure:"maximum_number_of_metrics_in_a_call"`
	AppendTagsAsAttributes                  bool                          `mapstructure:"append_tags_as_attributes"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (c Config) Validate() (err error) {
	if c.TenantID == "" {
		err = multierr.Append(err, errMissingTenantID)
	}

	if c.SubscriptionID == "" {
		err = multierr.Append(err, errMissingSubscriptionID)
	}

	if c.ClientID == "" {
		err = multierr.Append(err, errMissingClientID)
	}

	if c.ClientSecret == "" {
		err = multierr.Append(err, errMissingClientSecret)
	}

	return
}
