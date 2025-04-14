// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testShortHostname = "hostname"
	testCloudAccount  = "projectID"
	testHostname      = testShortHostname + ".c." + testCloudAccount + ".internal"
	testBadHostname   = "badhostname"
)

var (
	testGCPIntegrationHostname    = fmt.Sprintf("%s.%s", testShortHostname, testCloudAccount)
	testGCPIntegrationBadHostname = fmt.Sprintf("%s.%s", testBadHostname, testCloudAccount)
)

var _ gcpDetector = (*mockDetector)(nil)

type mockDetector struct {
	platform     gcp.Platform
	projectID    string
	instanceName string
}

func (m *mockDetector) CloudPlatform() gcp.Platform {
	return m.platform
}

func (m *mockDetector) ProjectID() (string, error) {
	return m.projectID, nil
}

func (m *mockDetector) GCEHostName() (string, error) {
	return m.instanceName, nil
}

func (m *mockDetector) GKEClusterName() (string, error) {
	return "", errors.New("not available")
}

func TestProvider(t *testing.T) {
	tests := []struct {
		name         string
		projectID    string
		platform     gcp.Platform
		instanceName string
		hostname     string
	}{
		{
			name:         "good hostname",
			platform:     gcp.GCE,
			projectID:    testCloudAccount,
			instanceName: testHostname,
			hostname:     testGCPIntegrationHostname,
		},
		{
			name:         "bad hostname",
			platform:     gcp.GKE,
			projectID:    testCloudAccount,
			instanceName: testBadHostname,
			hostname:     testGCPIntegrationBadHostname,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			provider := &Provider{detector: &mockDetector{
				platform:     testInstance.platform,
				projectID:    testInstance.projectID,
				instanceName: testInstance.instanceName,
			}}

			src, err := provider.Source(context.Background())
			require.NoError(t, err)
			assert.Equal(t, source.HostnameKind, src.Kind)
			assert.Equal(t, testInstance.hostname, src.Identifier)
		})
	}
}
