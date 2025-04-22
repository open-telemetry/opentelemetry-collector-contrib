// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

var referencableTrue = true

func TestValidate(t *testing.T) {
	testCases := []struct {
		name        string
		input       Config
		expectedErr string
	}{
		{
			name: "Empty config",
			input: Config{
				BaseURL:          mongodbatlas.CloudURL,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			name: "Valid alerts config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			name: "Alerts missing endpoint",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled: true,
					Secret:  "some_secret",
					Mode:    alertModeListen,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoEndpoint.Error(),
		},
		{
			name: "Alerts missing secret",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Mode:     alertModeListen,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoSecret.Error(),
		},
		{
			name: "Invalid endpoint",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: "failed to split endpoint into 'host:port' pair",
		},
		{
			name: "TLS config missing key",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
					TLS: &configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "some_cert_file",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoKey.Error(),
		},
		{
			name: "TLS config missing cert",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled:  true,
					Endpoint: "0.0.0.0:7706",
					Secret:   "some_secret",
					Mode:     alertModeListen,
					TLS: &configtls.ServerConfig{
						Config: configtls.Config{
							KeyFile: "some_key_file",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoCert.Error(),
		},
		{
			name: "Valid Metrics Config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Projects: []*ProjectConfig{
					{
						Name: "Project1",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			name: "Valid Metrics Config with multiple projects with an inclusion or exclusion",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Projects: []*ProjectConfig{
					{
						Name:            "Project1",
						IncludeClusters: []string{"Cluster1"},
					},
					{
						Name:            "Project2",
						ExcludeClusters: []string{"Cluster1"},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			name: "invalid Metrics Config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Projects: []*ProjectConfig{
					{
						Name:            "Project1",
						IncludeClusters: []string{"Cluster1"},
						ExcludeClusters: []string{"Cluster2"},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errClusterConfig.Error(),
		},
		{
			name: "Valid Logs Config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Logs: LogConfig{
					Enabled: true,
					Projects: []*LogsProjectConfig{
						{
							ProjectConfig: ProjectConfig{
								Name: "Project1",
							},
							EnableAuditLogs: false,
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			name: "Invalid Logs Config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Logs: LogConfig{
					Enabled: true,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoProjects.Error(),
		},
		{
			name: "Invalid ProjectConfig",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Logs: LogConfig{
					Enabled: true,
					Projects: []*LogsProjectConfig{
						{
							ProjectConfig: ProjectConfig{
								Name:            "Project1",
								ExcludeClusters: []string{"cluster1"},
								IncludeClusters: []string{"cluster2"},
							},
							EnableAuditLogs: false,
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errClusterConfig.Error(),
		},
		{
			name: "Invalid Alerts Retrieval ProjectConfig",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled: true,
					Mode:    alertModePoll,
					Projects: []*ProjectConfig{
						{
							Name:            "Project1",
							ExcludeClusters: []string{"cluster1"},
							IncludeClusters: []string{"cluster2"},
						},
					},
					PageSize: defaultAlertsPageSize,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errClusterConfig.Error(),
		},
		{
			name: "Invalid Alerts Poll No Projects",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled:  true,
					Mode:     alertModePoll,
					Projects: []*ProjectConfig{},
					PageSize: defaultAlertsPageSize,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoProjects.Error(),
		},
		{
			name: "Valid Alerts Config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled: true,
					Mode:    alertModePoll,
					Projects: []*ProjectConfig{
						{
							Name: "Project1",
						},
					},
					PageSize: defaultAlertsPageSize,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			name: "Invalid Alerts Mode",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled:  true,
					Mode:     "invalid type",
					Projects: []*ProjectConfig{},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoModeRecognized.Error(),
		},
		{
			name: "Invalid Page Size",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Alerts: AlertConfig{
					Enabled: true,
					Mode:    alertModePoll,
					Projects: []*ProjectConfig{
						{
							Name: "Test",
						},
					},
					PageSize: -1,
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errPageSizeIncorrect.Error(),
		},
		{
			name: "Invalid events config - no projects",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Events: &EventsConfig{
					Projects: []*ProjectConfig{},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errNoEvents.Error(),
		},
		{
			name: "Valid Access Logs Config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Logs: LogConfig{
					Projects: []*LogsProjectConfig{
						{
							ProjectConfig: ProjectConfig{
								Name:            "Project1",
								IncludeClusters: []string{"Cluster1"},
							},
							AccessLogs: &AccessLogsConfig{
								Enabled:    &referencableTrue,
								AuthResult: &referencableTrue,
							},
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			name: "Invalid Access Logs Config - bad project config",
			input: Config{
				BaseURL: mongodbatlas.CloudURL,
				Logs: LogConfig{
					Enabled: true,
					Projects: []*LogsProjectConfig{
						{
							ProjectConfig: ProjectConfig{
								Name:            "Project1",
								IncludeClusters: []string{"Cluster1"},
								ExcludeClusters: []string{"Cluster3"},
							},
							AccessLogs: &AccessLogsConfig{
								Enabled:    &referencableTrue,
								AuthResult: &referencableTrue,
							},
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errClusterConfig.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.MetricsBuilderConfig = metadata.DefaultMetricsBuilderConfig()
	expected.BaseURL = "https://cloud.mongodb.com/"
	expected.PrivateKey = "my-private-key"
	expected.PublicKey = "my-public-key"
	expected.Logs = LogConfig{
		Enabled: true,
		Projects: []*LogsProjectConfig{
			{
				ProjectConfig: ProjectConfig{
					Name: "Project 0",
				},
				AccessLogs: &AccessLogsConfig{
					Enabled:      &referencableTrue,
					AuthResult:   &referencableTrue,
					PollInterval: time.Minute,
				},
				EnableAuditLogs: true,
			},
		},
	}
	expected.Alerts = AlertConfig{
		Enabled: true,
		Mode:    alertModePoll,
		Projects: []*ProjectConfig{
			{
				Name:            "Project 0",
				IncludeClusters: []string{"Cluster0"},
			},
		},
		PageSize:     defaultAlertsPageSize,
		MaxPages:     defaultAlertsMaxPages,
		PollInterval: time.Minute,
	}

	expected.Events = &EventsConfig{
		Projects: []*ProjectConfig{
			{
				Name: "Project 0",
			},
		},
		Organizations: []*OrgConfig{
			{
				ID: "5b478b3afc4625789ce616a3",
			},
		},
		PollInterval: time.Minute,
		MaxPages:     defaultEventsMaxPages,
		PageSize:     defaultEventsPageSize,
	}
	require.Equal(t, expected, cfg)
}
