// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package gcp contains the GCP hostname provider
package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/gcp"

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/provider"
)

var (
	_ source.Provider              = (*Provider)(nil)
	_ provider.ClusterNameProvider = (*Provider)(nil)
)

var _ gcpDetector = (*contextDetector)(nil)

type gcpDetector interface {
	ProjectID(context.Context) (string, error)
	CloudPlatform(context.Context) gcp.Platform
	GCEHostName(context.Context) (string, error)
	GKEClusterName(context.Context) (string, error)
}

// contextDetector mirrors detectors/gcp with ctx propagation because gcp.NewDetector() passes context.TODO().
type contextDetector struct {
	metadata *metadata.Client
}

// CloudPlatform mirrors detectors/gcp.Detector.CloudPlatform precedence while propagating ctx.
func (d *contextDetector) CloudPlatform(ctx context.Context) gcp.Platform {
	hasEnv := func(name string) bool {
		_, ok := os.LookupEnv(name)
		return ok
	}
	hasMetadata := func(path string) bool {
		_, err := d.metadata.GetWithContext(ctx, path)
		return err == nil
	}
	switch {
	case os.Getenv("BMS_PROJECT_ID") != "" && os.Getenv("BMS_REGION") != "" && os.Getenv("BMS_INSTANCE_ID") != "":
		return gcp.BareMetalSolution
	case hasEnv("KUBERNETES_SERVICE_HOST") && hasMetadata("instance/attributes/cluster-location"):
		return gcp.GKE
	case ctx.Err() != nil:
		return gcp.UnknownPlatform
	case hasEnv("FUNCTION_TARGET"):
		return gcp.CloudFunctions
	case hasEnv("K_CONFIGURATION"):
		return gcp.CloudRun
	case hasEnv("CLOUD_RUN_JOB"):
		return gcp.CloudRunJob
	case hasEnv("CLOUD_RUN_WORKER_POOL"):
		return gcp.CloudRunWorkerPool
	case os.Getenv("GAE_ENV") == "standard":
		return gcp.AppEngineStandard
	case hasEnv("GAE_SERVICE"):
		return gcp.AppEngineFlex
	case hasMetadata("instance/machine-type"):
		return gcp.GCE
	}
	return gcp.UnknownPlatform
}

func (d *contextDetector) ProjectID(ctx context.Context) (string, error) {
	// Use GetWithContext instead of ProjectIDWithContext, which caches the project ID globally.
	s, err := d.metadata.GetWithContext(ctx, "project/project-id")
	return strings.TrimSpace(s), err
}

// GCEHostName mirrors detectors/gcp.Detector.GCEHostName, which returns the instance name rather than the hostname.
func (d *contextDetector) GCEHostName(ctx context.Context) (string, error) {
	return d.metadata.InstanceNameWithContext(ctx)
}

func (d *contextDetector) GKEClusterName(ctx context.Context) (string, error) {
	return d.metadata.InstanceAttributeValueWithContext(ctx, "cluster-name")
}

type Provider struct {
	detector gcpDetector
}

func platformDescription(platform gcp.Platform) string {
	switch platform {
	case gcp.UnknownPlatform:
		return "Unknown platform"
	case gcp.GKE:
		return "Google Kubernetes Engine"
	case gcp.GCE:
		return "Google Cloud Engine"
	case gcp.CloudRun:
		return "Google Cloud Run"
	case gcp.CloudFunctions:
		return "Google Cloud Functions"
	case gcp.AppEngineStandard, gcp.AppEngineFlex:
		return "Google AppEngine"
	case gcp.CloudRunJob:
		return "Cloud Run Job"
	}
	return "Unrecognized platform"
}

// Source returns the GCP cloud integration hostname.
func (p *Provider) Source(ctx context.Context) (source.Source, error) {
	if err := ctx.Err(); err != nil {
		return source.Source{}, err
	}
	if platform := p.detector.CloudPlatform(ctx); platform != gcp.GCE && platform != gcp.GKE {
		if err := ctx.Err(); err != nil {
			return source.Source{}, err
		}
		return source.Source{}, fmt.Errorf("not on GCE or GKE (platform: %s)", platformDescription(platform))
	}

	name, err := p.detector.GCEHostName(ctx)
	if err != nil {
		return source.Source{}, fmt.Errorf("failed to get instance name: %w", err)
	}

	// Use the same logic as in the metadata from attributes logic.
	if strings.Count(name, ".") >= 3 {
		name = strings.SplitN(name, ".", 2)[0]
	}

	cloudAccount, err := p.detector.ProjectID(ctx)
	if err != nil {
		return source.Source{}, fmt.Errorf("failed to get project ID: %w", err)
	}

	identifier := fmt.Sprintf("%s.%s", name, cloudAccount)
	return source.Source{Kind: source.HostnameKind, Identifier: identifier, SourceIdentifier: source.SourceIdentifier{Primary: identifier}}, nil
}

// ClusterName returns the GKE cluster name.
func (p *Provider) ClusterName(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return p.detector.GKEClusterName(ctx)
}

// NewProvider creates a new GCP hostname provider.
func NewProvider() *Provider {
	return &Provider{detector: &contextDetector{metadata: metadata.NewClient(nil)}}
}
