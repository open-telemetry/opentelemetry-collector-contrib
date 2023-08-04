// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"github.com/stretchr/testify/assert"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

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
				conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID:        "my-project",
				conventions.AttributeCloudPlatform:         conventions.AttributeCloudPlatformGCPKubernetesEngine,
				conventions.AttributeK8SClusterName:        "my-cluster",
				conventions.AttributeCloudAvailabilityZone: "us-central1-c",
				conventions.AttributeHostID:                "1472385723456792345",
				conventions.AttributeHostName:              "my-gke-node-1234",
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
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID: "my-project",
				conventions.AttributeCloudPlatform:  conventions.AttributeCloudPlatformGCPKubernetesEngine,
				conventions.AttributeK8SClusterName: "my-cluster",
				conventions.AttributeCloudRegion:    "us-central1",
				conventions.AttributeHostID:         "1472385723456792345",
				conventions.AttributeHostName:       "my-gke-node-1234",
			},
		},
		{
			desc: "regional GKE cluster with workload identity",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:      "my-project",
				cloudPlatform:  gcp.GKE,
				gceHostNameErr: fmt.Errorf("metadata endpoint is concealed"),
				gkeHostID:      "1472385723456792345",
				gkeClusterName: "my-cluster",
				gkeRegion:      "us-central1",
			}),
			expectedResource: map[string]any{
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID: "my-project",
				conventions.AttributeCloudPlatform:  conventions.AttributeCloudPlatformGCPKubernetesEngine,
				conventions.AttributeK8SClusterName: "my-cluster",
				conventions.AttributeCloudRegion:    "us-central1",
				conventions.AttributeHostID:         "1472385723456792345",
			},
		},
		{
			desc: "GCE",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:           "my-project",
				cloudPlatform:       gcp.GCE,
				gceHostID:           "1472385723456792345",
				gceHostName:         "my-gke-node-1234",
				gceHostType:         "n1-standard1",
				gceAvailabilityZone: "us-central1-c",
				gceRegion:           "us-central1",
			}),
			expectedResource: map[string]any{
				conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID:        "my-project",
				conventions.AttributeCloudPlatform:         conventions.AttributeCloudPlatformGCPComputeEngine,
				conventions.AttributeHostID:                "1472385723456792345",
				conventions.AttributeHostName:              "my-gke-node-1234",
				conventions.AttributeHostType:              "n1-standard1",
				conventions.AttributeCloudRegion:           "us-central1",
				conventions.AttributeCloudAvailabilityZone: "us-central1-c",
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
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID: "my-project",
				conventions.AttributeCloudPlatform:  conventions.AttributeCloudPlatformGCPCloudRun,
				conventions.AttributeCloudRegion:    "us-central1",
				conventions.AttributeFaaSName:       "my-service",
				conventions.AttributeFaaSVersion:    "123456",
				conventions.AttributeFaaSID:         "1472385723456792345",
			},
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
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID: "my-project",
				conventions.AttributeCloudPlatform:  conventions.AttributeCloudPlatformGCPCloudRun,
				conventions.AttributeCloudRegion:    "us-central1",
				conventions.AttributeFaaSName:       "my-service",
				conventions.AttributeFaaSID:         "1472385723456792345",
				"gcp.cloud_run.job.execution":       "my-service-ajg89",
				"gcp.cloud_run.job.task_index":      "2",
			},
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
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID: "my-project",
				conventions.AttributeCloudPlatform:  conventions.AttributeCloudPlatformGCPCloudFunctions,
				conventions.AttributeCloudRegion:    "us-central1",
				conventions.AttributeFaaSName:       "my-service",
				conventions.AttributeFaaSVersion:    "123456",
				conventions.AttributeFaaSID:         "1472385723456792345",
			},
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
				conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID:        "my-project",
				conventions.AttributeCloudPlatform:         conventions.AttributeCloudPlatformGCPAppEngine,
				conventions.AttributeCloudRegion:           "us-central1",
				conventions.AttributeCloudAvailabilityZone: "us-central1-c",
				conventions.AttributeFaaSName:              "my-service",
				conventions.AttributeFaaSVersion:           "123456",
				conventions.AttributeFaaSID:                "1472385723456792345",
			},
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
				conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID:        "my-project",
				conventions.AttributeCloudPlatform:         conventions.AttributeCloudPlatformGCPAppEngine,
				conventions.AttributeCloudRegion:           "us-central1",
				conventions.AttributeCloudAvailabilityZone: "us-central1-c",
				conventions.AttributeFaaSName:              "my-service",
				conventions.AttributeFaaSVersion:           "123456",
				conventions.AttributeFaaSID:                "1472385723456792345",
			},
		},
		{
			desc: "Unknown Platform",
			detector: newTestDetector(&fakeGCPDetector{
				projectID:     "my-project",
				cloudPlatform: gcp.UnknownPlatform,
			}),
			expectedResource: map[string]any{
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID: "my-project",
			},
		},
		{
			desc: "error",
			detector: newTestDetector(&fakeGCPDetector{
				err: fmt.Errorf("failed to get metadata"),
			}),
			expectErr: true,
			expectedResource: map[string]any{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP,
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			res, schema, err := tc.detector.Detect(context.TODO())
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, conventions.SchemaURL, schema)
			assert.Equal(t, tc.expectedResource, res.Attributes().AsRaw(), "Resource object returned is incorrect")
		})
	}
}

func newTestDetector(gcpDetector *fakeGCPDetector) *detector {
	return &detector{
		logger:   zap.NewNop(),
		detector: gcpDetector,
		rb:       localMetadata.NewResourceBuilder(localMetadata.DefaultResourceAttributesConfig()),
	}
}

// fakeGCPDetector implements gcpDetector and uses fake values.
type fakeGCPDetector struct {
	err                       error
	projectID                 string
	cloudPlatform             gcp.Platform
	gkeAvailabilityZone       string
	gkeRegion                 string
	gkeClusterName            string
	gkeHostID                 string
	faaSName                  string
	faaSVersion               string
	faaSID                    string
	faaSCloudRegion           string
	appEngineAvailabilityZone string
	appEngineRegion           string
	appEngineServiceName      string
	appEngineServiceVersion   string
	appEngineServiceInstance  string
	gceAvailabilityZone       string
	gceRegion                 string
	gceHostType               string
	gceHostID                 string
	gceHostName               string
	gceHostNameErr            error
	gcpCloudRunJobExecution   string
	gcpCloudRunJobTaskIndex   string
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
