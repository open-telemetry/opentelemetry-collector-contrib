// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"errors"
	"testing"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	localMetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
)

func TestDetect(t *testing.T) {
	// Set this before all tests to ensure metadata.onGCE() returns true
	t.Setenv("GCE_METADATA_HOST", "169.254.169.254")

	for _, tc := range []struct {
		desc             string
		detector         internal.Detector
		expectErr        bool
		expectedResource map[string]any
		addFaasID        bool
	}{
		{
			desc: "zonal GKE cluster",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:           "my-project",
				cloudPlatform:       gcp.GKE,
				gceHostName:         "my-gke-node-1234",
				gkeHostID:           "1472385723456792345",
				gkeClusterName:      "my-cluster",
				gkeAvailabilityZone: "us-central1-c",
			}),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_kubernetes_engine",
				"k8s.cluster.name":        "my-cluster",
				"cloud.availability_zone": "us-central1-c",
				"host.id":                 "1472385723456792345",
				"host.name":               "my-gke-node-1234",
			},
		},
		{
			desc: "regional GKE cluster",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:      "my-project",
				cloudPlatform:  gcp.GKE,
				gceHostName:    "my-gke-node-1234",
				gkeHostID:      "1472385723456792345",
				gkeClusterName: "my-cluster",
				gkeRegion:      "us-central1",
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_kubernetes_engine",
				"k8s.cluster.name": "my-cluster",
				"cloud.region":     "us-central1",
				"host.id":          "1472385723456792345",
				"host.name":        "my-gke-node-1234",
			},
		},
		{
			desc: "regional GKE cluster with workload identity",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:      "my-project",
				cloudPlatform:  gcp.GKE,
				gceHostNameErr: errors.New("metadata endpoint is concealed"),
				gkeHostID:      "1472385723456792345",
				gkeClusterName: "my-cluster",
				gkeRegion:      "us-central1",
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_kubernetes_engine",
				"k8s.cluster.name": "my-cluster",
				"cloud.region":     "us-central1",
				"host.id":          "1472385723456792345",
			},
		},
		{
			desc: "GCE",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:              "my-project",
				cloudPlatform:          gcp.GCE,
				gceHostID:              "1472385723456792345",
				gceHostName:            "my-gke-node-1234",
				gceHostType:            "n1-standard1",
				gceAvailabilityZone:    "us-central1-c",
				gceRegion:              "us-central1",
				gcpGceInstanceHostname: "custom.dns.example.com",
				gcpGceInstanceName:     "my-gke-node-1234",
			}),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_compute_engine",
				"host.id":                 "1472385723456792345",
				"host.name":               "my-gke-node-1234",
				"host.type":               "n1-standard1",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
			},
		},
		{
			desc: "GCE with instance.hostname and instance.name enabled",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:              "my-project",
				cloudPlatform:          gcp.GCE,
				gceHostID:              "1472385723456792345",
				gceHostName:            "my-gke-node-1234",
				gceHostType:            "n1-standard1",
				gceAvailabilityZone:    "us-central1-c",
				gceRegion:              "us-central1",
				gcpGceInstanceHostname: "custom.dns.example.com",
				gcpGceInstanceName:     "my-gke-node-1234",
			}, func(cfg *localMetadata.ResourceAttributesConfig) {
				cfg.GcpGceInstanceHostname.Enabled = true
				cfg.GcpGceInstanceName.Enabled = true
			}),
			expectedResource: map[string]any{
				"cloud.provider":            "gcp",
				"cloud.account.id":          "my-project",
				"cloud.platform":            "gcp_compute_engine",
				"host.id":                   "1472385723456792345",
				"host.name":                 "my-gke-node-1234",
				"host.type":                 "n1-standard1",
				"cloud.region":              "us-central1",
				"cloud.availability_zone":   "us-central1-c",
				"gcp.gce.instance.hostname": "custom.dns.example.com",
				"gcp.gce.instance.name":     "my-gke-node-1234",
			},
		},
		{
			desc: "GCE with MIG",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:              "my-project",
				cloudPlatform:          gcp.GCE,
				gceHostID:              "1472385723456792345",
				gceHostName:            "my-gke-node-1234",
				gceHostType:            "n1-standard1",
				gceAvailabilityZone:    "us-central1-c",
				gceRegion:              "us-central1",
				gcpGceInstanceHostname: "custom.dns.example.com",
				gcpGceInstanceName:     "my-gke-node-1234",
				gcpGceManagedInstanceGroup: gcp.ManagedInstanceGroup{
					Name:     "my-gke-node",
					Location: "us-central1",
					Type:     gcp.Region,
				},
			}),
			expectedResource: map[string]any{
				"cloud.provider":                        "gcp",
				"cloud.account.id":                      "my-project",
				"cloud.platform":                        "gcp_compute_engine",
				"host.id":                               "1472385723456792345",
				"host.name":                             "my-gke-node-1234",
				"host.type":                             "n1-standard1",
				"cloud.region":                          "us-central1",
				"cloud.availability_zone":               "us-central1-c",
				"gcp.gce.instance_group_manager.name":   "my-gke-node",
				"gcp.gce.instance_group_manager.region": "us-central1",
			},
		},
		{
			desc: "Cloud Run",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:       "my-project",
				cloudPlatform:   gcp.CloudRun,
				faaSID:          "1472385723456792345",
				faaSCloudRegion: "us-central1",
				faaSName:        "my-service",
				faaSVersion:     "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_cloud_run",
				"cloud.region":     "us-central1",
				"faas.name":        "my-service",
				"faas.version":     "123456",
				"faas.instance":    "1472385723456792345",
			},
		},
		{
			desc: "Cloud Run with feature gate disabled",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:       "my-project",
				cloudPlatform:   gcp.CloudRun,
				faaSID:          "1472385723456792345",
				faaSCloudRegion: "us-central1",
				faaSName:        "my-service",
				faaSVersion:     "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_cloud_run",
				"cloud.region":     "us-central1",
				"faas.name":        "my-service",
				"faas.version":     "123456",
				"faas.instance":    "1472385723456792345",
				"faas.id":          "1472385723456792345",
			},
			addFaasID: true,
		},
		{
			desc: "Cloud Run Job",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:               "my-project",
				cloudPlatform:           gcp.CloudRunJob,
				faaSID:                  "1472385723456792345",
				faaSCloudRegion:         "us-central1",
				faaSName:                "my-service",
				gcpCloudRunJobExecution: "my-service-ajg89",
				gcpCloudRunJobTaskIndex: "2",
			}),
			expectedResource: map[string]any{
				"cloud.provider":               "gcp",
				"cloud.account.id":             "my-project",
				"cloud.platform":               "gcp_cloud_run",
				"cloud.region":                 "us-central1",
				"faas.name":                    "my-service",
				"faas.instance":                "1472385723456792345",
				"gcp.cloud_run.job.execution":  "my-service-ajg89",
				"gcp.cloud_run.job.task_index": "2",
			},
		},
		{
			desc: "Cloud Run Job with feature gate disabled",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:               "my-project",
				cloudPlatform:           gcp.CloudRunJob,
				faaSID:                  "1472385723456792345",
				faaSCloudRegion:         "us-central1",
				faaSName:                "my-service",
				gcpCloudRunJobExecution: "my-service-ajg89",
				gcpCloudRunJobTaskIndex: "2",
			}),
			expectedResource: map[string]any{
				"cloud.provider":               "gcp",
				"cloud.account.id":             "my-project",
				"cloud.platform":               "gcp_cloud_run",
				"cloud.region":                 "us-central1",
				"faas.name":                    "my-service",
				"faas.instance":                "1472385723456792345",
				"faas.id":                      "1472385723456792345",
				"gcp.cloud_run.job.execution":  "my-service-ajg89",
				"gcp.cloud_run.job.task_index": "2",
			},
			addFaasID: true,
		},
		{
			desc: "Cloud Functions",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:       "my-project",
				cloudPlatform:   gcp.CloudFunctions,
				faaSID:          "1472385723456792345",
				faaSCloudRegion: "us-central1",
				faaSName:        "my-service",
				faaSVersion:     "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_cloud_functions",
				"cloud.region":     "us-central1",
				"faas.name":        "my-service",
				"faas.version":     "123456",
				"faas.instance":    "1472385723456792345",
			},
		},
		{
			desc: "Cloud Functions with feature gate disabled",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:       "my-project",
				cloudPlatform:   gcp.CloudFunctions,
				faaSID:          "1472385723456792345",
				faaSCloudRegion: "us-central1",
				faaSName:        "my-service",
				faaSVersion:     "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_cloud_functions",
				"cloud.region":     "us-central1",
				"faas.name":        "my-service",
				"faas.version":     "123456",
				"faas.instance":    "1472385723456792345",
				"faas.id":          "1472385723456792345",
			},
			addFaasID: true,
		},
		{
			desc: "App Engine Standard",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:                 "my-project",
				cloudPlatform:             gcp.AppEngineStandard,
				appEngineServiceInstance:  "1472385723456792345",
				appEngineAvailabilityZone: "us-central1-c",
				appEngineRegion:           "us-central1",
				appEngineServiceName:      "my-service",
				appEngineServiceVersion:   "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_app_engine",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
				"faas.name":               "my-service",
				"faas.version":            "123456",
				"faas.instance":           "1472385723456792345",
			},
		},
		{
			desc: "App Engine Standard with feature gate disabled",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:                 "my-project",
				cloudPlatform:             gcp.AppEngineStandard,
				appEngineServiceInstance:  "1472385723456792345",
				appEngineAvailabilityZone: "us-central1-c",
				appEngineRegion:           "us-central1",
				appEngineServiceName:      "my-service",
				appEngineServiceVersion:   "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_app_engine",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
				"faas.name":               "my-service",
				"faas.version":            "123456",
				"faas.instance":           "1472385723456792345",
				"faas.id":                 "1472385723456792345",
			},
			addFaasID: true,
		},
		{
			desc: "App Engine Flex",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:                 "my-project",
				cloudPlatform:             gcp.AppEngineFlex,
				appEngineServiceInstance:  "1472385723456792345",
				appEngineAvailabilityZone: "us-central1-c",
				appEngineRegion:           "us-central1",
				appEngineServiceName:      "my-service",
				appEngineServiceVersion:   "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_app_engine",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
				"faas.name":               "my-service",
				"faas.version":            "123456",
				"faas.instance":           "1472385723456792345",
			},
		},
		{
			desc: "App Engine Flex with feature gate disabled",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:                 "my-project",
				cloudPlatform:             gcp.AppEngineFlex,
				appEngineServiceInstance:  "1472385723456792345",
				appEngineAvailabilityZone: "us-central1-c",
				appEngineRegion:           "us-central1",
				appEngineServiceName:      "my-service",
				appEngineServiceVersion:   "123456",
			}),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_app_engine",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
				"faas.name":               "my-service",
				"faas.version":            "123456",
				"faas.instance":           "1472385723456792345",
				"faas.id":                 "1472385723456792345",
			},
			addFaasID: true,
		},
		{
			desc: "Bare Metal Solution",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:                       "my-project",
				cloudPlatform:                   gcp.BareMetalSolution,
				gcpBareMetalSolutionCloudRegion: "us-central1",
				gcpBareMetalSolutionInstanceID:  "1472385723456792345",
				gcpBareMetalSolutionProjectID:   "my-project",
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_bare_metal_solution",
				"cloud.region":     "us-central1",
				"host.name":        "1472385723456792345",
			},
		},
		{
			desc: "Unknown Platform",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:     "my-project",
				cloudPlatform: gcp.UnknownPlatform,
			}),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
			},
		},
		{
			desc: "error",
			detector: newTestDetector(&fakeGCPDetector{
				err: errors.New("failed to get metadata"),
			}),
			expectErr: true,
			expectedResource: map[string]any{
				"cloud.provider": "gcp",
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			defer testutil.SetFeatureGateForTest(t, removeGCPFaasID, !tc.addFaasID)()
			res, schema, err := tc.detector.Detect(t.Context())
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			assert.Equal(t, tc.expectedResource, res.Attributes().AsRaw(), "Resource object returned is incorrect")
		})
	}
}

func newTestDetector(gcpDetector *fakeGCPDetector, opts ...func(*localMetadata.ResourceAttributesConfig)) *detector {
	cfg := localMetadata.DefaultResourceAttributesConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &detector{
		logger:   zap.NewNop(),
		detector: gcpDetector,
		rb:       localMetadata.NewResourceBuilder(cfg),
	}
}

// fakeGCPDetector implements gcpDetector and uses fake values.
type fakeGCPDetector struct {
	err                             error
	projectID                       string
	cloudPlatform                   gcp.Platform
	gkeAvailabilityZone             string
	gkeRegion                       string
	gkeClusterName                  string
	gkeHostID                       string
	faaSName                        string
	faaSVersion                     string
	faaSID                          string
	faaSCloudRegion                 string
	appEngineAvailabilityZone       string
	appEngineRegion                 string
	appEngineServiceName            string
	appEngineServiceVersion         string
	appEngineServiceInstance        string
	gceAvailabilityZone             string
	gceRegion                       string
	gceHostType                     string
	gceHostID                       string
	gceHostName                     string
	gceHostNameErr                  error
	gcpCloudRunJobExecution         string
	gcpCloudRunJobTaskIndex         string
	gcpGceInstanceName              string
	gcpGceInstanceHostname          string
	gcpGceManagedInstanceGroup      gcp.ManagedInstanceGroup
	gcpBareMetalSolutionInstanceID  string
	gcpBareMetalSolutionCloudRegion string
	gcpBareMetalSolutionProjectID   string
}

func (f *fakeGCPDetector) ProjectID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.projectID, nil
}

func (f *fakeGCPDetector) CloudPlatform() gcp.Platform {
	return f.cloudPlatform
}

func (f *fakeGCPDetector) GKEAvailabilityZoneOrRegion() (string, gcp.LocationType, error) {
	if f.err != nil {
		return "", gcp.UndefinedLocation, f.err
	}
	if f.gkeAvailabilityZone != "" {
		return f.gkeAvailabilityZone, gcp.Zone, nil
	}
	return f.gkeRegion, gcp.Region, nil
}

func (f *fakeGCPDetector) GKEClusterName() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gkeClusterName, nil
}

func (f *fakeGCPDetector) GKEHostID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gkeHostID, nil
}

func (f *fakeGCPDetector) FaaSName() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.faaSName, nil
}

func (f *fakeGCPDetector) FaaSVersion() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.faaSVersion, nil
}

func (f *fakeGCPDetector) FaaSID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.faaSID, nil
}

func (f *fakeGCPDetector) FaaSCloudRegion() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.faaSCloudRegion, nil
}

func (f *fakeGCPDetector) AppEngineFlexAvailabilityZoneAndRegion() (string, string, error) {
	if f.err != nil {
		return "", "", f.err
	}
	return f.appEngineAvailabilityZone, f.appEngineRegion, nil
}

func (f *fakeGCPDetector) AppEngineStandardAvailabilityZone() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.appEngineAvailabilityZone, nil
}

func (f *fakeGCPDetector) AppEngineStandardCloudRegion() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.appEngineRegion, nil
}

func (f *fakeGCPDetector) AppEngineServiceName() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.appEngineServiceName, nil
}

func (f *fakeGCPDetector) AppEngineServiceVersion() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.appEngineServiceVersion, nil
}

func (f *fakeGCPDetector) AppEngineServiceInstance() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.appEngineServiceInstance, nil
}

func (f *fakeGCPDetector) GCEAvailabilityZoneAndRegion() (string, string, error) {
	if f.err != nil {
		return "", "", f.err
	}
	return f.gceAvailabilityZone, f.gceRegion, nil
}

func (f *fakeGCPDetector) GCEHostType() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gceHostType, nil
}

func (f *fakeGCPDetector) GCEHostID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gceHostID, nil
}

func (f *fakeGCPDetector) GCEHostName() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gceHostName, f.gceHostNameErr
}

func (f *fakeGCPDetector) CloudRunJobTaskIndex() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gcpCloudRunJobTaskIndex, nil
}

func (f *fakeGCPDetector) CloudRunJobExecution() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gcpCloudRunJobExecution, nil
}

func (f *fakeGCPDetector) GCEInstanceName() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gcpGceInstanceName, nil
}

func (f *fakeGCPDetector) GCEInstanceHostname() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gcpGceInstanceHostname, nil
}

func (f *fakeGCPDetector) GCEManagedInstanceGroup() (gcp.ManagedInstanceGroup, error) {
	if f.err != nil {
		return gcp.ManagedInstanceGroup{}, f.err
	}
	return f.gcpGceManagedInstanceGroup, nil
}

func (f *fakeGCPDetector) BareMetalSolutionInstanceID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gcpBareMetalSolutionInstanceID, nil
}

func (f *fakeGCPDetector) BareMetalSolutionCloudRegion() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gcpBareMetalSolutionCloudRegion, nil
}

func (f *fakeGCPDetector) BareMetalSolutionProjectID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.gcpBareMetalSolutionProjectID, nil
}
