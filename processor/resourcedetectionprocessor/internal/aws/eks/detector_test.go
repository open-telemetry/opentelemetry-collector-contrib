// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	apiprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

type mockIMDSProvider struct {
	meta        imds.InstanceIdentityDocument
	hostname    string
	hostnameErr error
	err         error
}

func (m *mockIMDSProvider) Get(_ context.Context) (imds.InstanceIdentityDocument, error) {
	return m.meta, m.err
}

func (m *mockIMDSProvider) Hostname(_ context.Context) (string, error) {
	if m.hostnameErr != nil {
		return "", m.hostnameErr
	}
	return m.hostname, m.err
}

func (m *mockIMDSProvider) InstanceID(_ context.Context) (string, error) {
	return m.meta.InstanceID, m.err
}

func (m *mockIMDSProvider) Tags(_ context.Context) ([]string, error) {
	return nil, m.err
}

func (m *mockIMDSProvider) Tag(_ context.Context, _ string) (string, error) {
	return "", m.err
}

type mockAPIProvider struct {
	clusterVersion    string
	clusterVersionErr error
	oidcIssuer        string
	oidcIssuerErr     error
	k8sMeta           apiprovider.InstanceMetadata
	k8sErr            error
	apiMeta           apiprovider.InstanceMetadata
	apiErr            error
	nodeName          string
	clusterARN        string
	clusterARNErr     error
	clusterARNLookups []string
}

func (m *mockAPIProvider) ClusterVersion() (string, error) {
	return m.clusterVersion, m.clusterVersionErr
}

func (m *mockAPIProvider) OIDCIssuer(_ context.Context) (string, error) {
	return m.oidcIssuer, m.oidcIssuerErr
}

func (m *mockAPIProvider) GetK8sInstanceMetadata(_ context.Context) (apiprovider.InstanceMetadata, error) {
	if m.nodeName == "" {
		return apiprovider.InstanceMetadata{}, errors.New("can't get K8s Instance Metadata; node name is empty")
	}
	return m.k8sMeta, m.k8sErr
}

func (m *mockAPIProvider) GetInstanceMetadata(ctx context.Context) (apiprovider.InstanceMetadata, error) {
	if m.apiMeta.Region == "" || m.apiMeta.InstanceID == "" {
		k8sMeta, err := m.GetK8sInstanceMetadata(ctx)
		if err != nil {
			return apiprovider.InstanceMetadata{}, err
		}
		m.apiMeta = apiprovider.InstanceMetadata{
			AvailabilityZone: k8sMeta.AvailabilityZone,
			Region:           k8sMeta.Region,
			InstanceID:       k8sMeta.InstanceID,
		}
	}
	return m.apiMeta, m.apiErr
}

func (m *mockAPIProvider) ClusterARN(_ context.Context, clusterName string) (string, error) {
	m.clusterARNLookups = append(m.clusterARNLookups, clusterName)
	return m.clusterARN, m.clusterARNErr
}

func (m *mockAPIProvider) SetRegionInstanceID(region, instanceID string) {
	m.apiMeta.Region = region
	m.apiMeta.InstanceID = instanceID
}

func TestIsEKS(t *testing.T) {
	tests := []struct {
		name              string
		k8sHostEnvVar     string
		irsaTokenFile     string
		podIdentityFile   string
		oidcIssuer        string
		oidcIssuerErr     error
		clusterVersion    string
		clusterVersionErr error
		isEKS             bool
		err               string
	}{
		{
			name:          "Not K8s Cluster",
			k8sHostEnvVar: "",
			isEKS:         false,
		},
		{
			name:          "IRSA Token Path",
			k8sHostEnvVar: "1",
			irsaTokenFile: "/var/run/secrets/eks.amazonaws.com/serviceaccount/token",
			isEKS:         true,
		},
		{
			name:            "Pod Identity Token Path",
			k8sHostEnvVar:   "1",
			podIdentityFile: "/var/run/secrets/eks-pod-identity/serviceaccount/token",
			isEKS:           true,
		},
		{
			name:          "OIDC Issuer EKS",
			k8sHostEnvVar: "1",
			oidcIssuer:    "https://oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B716D3041E",
			isEKS:         true,
		},
		{
			name:           "OIDC Issuer Error Falls Through",
			k8sHostEnvVar:  "1",
			oidcIssuerErr:  errors.New("fail"),
			clusterVersion: "v1.32.3-eks-d0fe756",
			isEKS:          true,
		},
		{
			name:              "ClusterVersion Error",
			k8sHostEnvVar:     "1",
			clusterVersionErr: errors.New("fail"),
			isEKS:             false,
			err:               "isEks() error retrieving cluster version:",
		},
		{
			name:           "ClusterVersion Not EKS",
			k8sHostEnvVar:  "1",
			clusterVersion: "v1.25.0",
			isEKS:          false,
		},
		{
			name:           "ClusterVersion EKS",
			k8sHostEnvVar:  "1",
			clusterVersion: "v1.32.3-eks-d0fe756",
			isEKS:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(kubernetesServiceHostEnvVar, tt.k8sHostEnvVar)
			t.Setenv(awsWebIdentityTokenFileEnvVar, tt.irsaTokenFile)
			t.Setenv(awsContainerAuthTokenFileEnvVar, tt.podIdentityFile)

			d := &detector{
				apiProvider: &mockAPIProvider{
					clusterVersion:    tt.clusterVersion,
					clusterVersionErr: tt.clusterVersionErr,
					oidcIssuer:        tt.oidcIssuer,
					oidcIssuerErr:     tt.oidcIssuerErr,
				},
			}
			isEKS, err := d.isEKS(t.Context())
			if tt.err != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.isEKS, isEKS)
		})
	}
}

func TestDetectFromIMDS(t *testing.T) {
	tests := []struct {
		name                  string
		imdsMeta              imds.InstanceIdentityDocument
		apiMeta               apiprovider.InstanceMetadata
		hostname              string
		hostnameErr           error
		apiErr                error
		imdsErr               error
		failOnMissingMetadata bool
		expectedError         bool
		errMsg                string
		expectedOutput        map[string]any
	}{
		{
			name: "Success",
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			expectedError: false,
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			},
		},
		{
			name:                  "IMDS Failure - fail_on_missing_metadata",
			imdsMeta:              imds.InstanceIdentityDocument{},
			apiMeta:               apiprovider.InstanceMetadata{},
			hostname:              "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			imdsErr:               errors.New("fail"),
			failOnMissingMetadata: true,
			expectedError:         true,
		},
		{
			// Default config: a detector must not report success and failure at once, so
			// the error is swallowed and only what was gathered is returned.
			name:          "IMDS Failure - suppressed",
			imdsMeta:      imds.InstanceIdentityDocument{},
			apiMeta:       apiprovider.InstanceMetadata{},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			imdsErr:       errors.New("fail"),
			expectedError: false,
		},
		{
			name: "Hostname Failure - fail_on_missing_metadata",
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
			},
			hostnameErr:           errors.New("fail"),
			failOnMissingMetadata: true,
			expectedError:         true,
			errMsg:                "fail",
		},
		{
			name: "Hostname Failure - suppressed",
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
			},
			hostnameErr:   errors.New("fail"),
			expectedError: false,
			expectedOutput: map[string]any{
				"host.id": "i-123",
			},
		},
		{
			name: "API Failure - Partial Result",
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			apiErr:        errors.New("fail"),
			expectedError: false,
			expectedOutput: map[string]any{
				"host.id":   "i-123",
				"host.name": "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			},
		},
		{
			name: "API Failure - fail_on_missing_metadata",
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:              "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			apiErr:                errors.New("fail"),
			failOnMissingMetadata: true,
			expectedError:         true,
		},
		{
			name: "API Failure - Missing Region/InstanceID",
			imdsMeta: imds.InstanceIdentityDocument{
				AvailabilityZone: "us-west-2a",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:      "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			expectedError: false,
			expectedOutput: map[string]any{
				"host.id":   "",
				"host.name": "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			},
		},
		{
			name: "API Failure - Missing Region/InstanceID - fail_on_missing_metadata",
			imdsMeta: imds.InstanceIdentityDocument{
				AvailabilityZone: "us-west-2a",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName: "cluster",
			},
			hostname:              "hostname.ip-192-168-1-1.us-west-2.compute.internal",
			failOnMissingMetadata: true,
			expectedError:         true,
			errMsg:                "can't get K8s Instance Metadata; node name is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &detector{
				logger:       zaptest.NewLogger(t),
				imdsProvider: &mockIMDSProvider{meta: tt.imdsMeta, hostname: tt.hostname, hostnameErr: tt.hostnameErr, err: tt.imdsErr},
				apiProvider:  &mockAPIProvider{apiMeta: tt.apiMeta, apiErr: tt.apiErr},
				rb: metadata.NewResourceBuilder(metadata.ResourceAttributesConfig{
					K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
					HostName:       metadata.ResourceAttributeConfig{Enabled: true},
					HostID:         metadata.ResourceAttributeConfig{Enabled: true},
				}),
				ra:                    metadata.ResourceAttributesConfig{K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true}},
				failOnMissingMetadata: tt.failOnMissingMetadata,
			}
			res, schema, err := d.detectFromIMDS(t.Context())
			if tt.expectedError {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
				assert.Empty(t, schema)
				assert.Equal(t, 0, res.Attributes().Len())
				return
			}
			require.NoError(t, err)
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			if len(tt.expectedOutput) > 0 {
				assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
			} else {
				assert.Equal(t, 0, res.Attributes().Len())
			}
		})
	}
}

func TestDetectFromAPI(t *testing.T) {
	tests := []struct {
		name                  string
		nodeName              string
		k8sMeta               apiprovider.InstanceMetadata
		apiMeta               apiprovider.InstanceMetadata
		k8sErr                error
		apiErr                error
		failOnMissingMetadata bool
		expectedError         bool
		expectedOutput        map[string]any
	}{
		{
			name:     "Success",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
				ClusterName:      "cluster",
				Hostname:         "i-123",
			},
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "i-123",
			},
		},
		{
			name:     "K8s Metadata Error",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
				ClusterName:      "cluster",
			},
			k8sErr:                errors.New("fail"),
			failOnMissingMetadata: true,
			expectedError:         true,
		},
		{
			name:     "K8s Metadata Error - suppressed",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
				ClusterName:      "cluster",
			},
			k8sErr:        errors.New("fail"),
			expectedError: false,
		},
		{
			name:     "API Metadata Error - fail_on_missing_metadata",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta:               apiprovider.InstanceMetadata{},
			apiErr:                errors.New("fail"),
			failOnMissingMetadata: true,
			expectedError:         true,
		},
		{
			name:     "API Metadata Error - Partial Result K8s Metadata",
			nodeName: "i-123",
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
			apiMeta:       apiprovider.InstanceMetadata{},
			apiErr:        errors.New("fail"),
			expectedError: false,
			expectedOutput: map[string]any{
				"host.id": "i-123",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &detector{
				logger: zaptest.NewLogger(t),
				apiProvider: &mockAPIProvider{
					k8sMeta:  tt.k8sMeta,
					apiMeta:  tt.apiMeta,
					k8sErr:   tt.k8sErr,
					apiErr:   tt.apiErr,
					nodeName: tt.nodeName,
				},
				rb: metadata.NewResourceBuilder(metadata.ResourceAttributesConfig{
					K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
					HostName:       metadata.ResourceAttributeConfig{Enabled: true},
					HostID:         metadata.ResourceAttributeConfig{Enabled: true},
				}),
				failOnMissingMetadata: tt.failOnMissingMetadata,
			}
			res, schema, err := d.detectFromAPI(t.Context())
			if tt.expectedError {
				require.Error(t, err)
				assert.Empty(t, schema)
				assert.Equal(t, 0, res.Attributes().Len())
				return
			}
			require.NoError(t, err)
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			if len(tt.expectedOutput) > 0 {
				assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
			} else {
				assert.Equal(t, 0, res.Attributes().Len())
			}
		})
	}
}

func TestValidClusterName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expected    bool
	}{
		{name: "simple", clusterName: "my-cluster", expected: true},
		{name: "separators", clusterName: "my-cluster_name", expected: true},
		{name: "mixed case and digits", clusterName: "MyCluster2", expected: true},
		{name: "100 characters", clusterName: strings.Repeat("a", 100), expected: true},
		{name: "single character", clusterName: "a", expected: true},
		{name: "trailing separator", clusterName: "my-cluster-", expected: true},
		{name: "empty", clusterName: "", expected: false},
		{name: "starts with a separator", clusterName: "-my-cluster", expected: false},
		{name: "only a separator", clusterName: "_", expected: false},
		{name: "contains a slash", clusterName: "team/cluster", expected: false},
		{name: "contains a colon", clusterName: "team:cluster", expected: false},
		{name: "exceeds 100 characters", clusterName: strings.Repeat("a", 101), expected: false},
		// The limit EKS documents is 100 characters. These names are rejected
		// on the character they contain, not on their byte length, which is
		// what keeps the byte-length check above equal to that limit.
		{name: "starts with a multi-byte character", clusterName: "é-cluster", expected: false},
		{name: "contains a multi-byte character", clusterName: "my-clusteré", expected: false},
		{name: "100 multi-byte characters", clusterName: strings.Repeat("é", 100), expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, validClusterName(tt.clusterName))
		})
	}
}

func TestDetectClusterARNFromIMDS(t *testing.T) {
	tests := []struct {
		name            string
		ra              metadata.ResourceAttributesConfig
		imdsMeta        imds.InstanceIdentityDocument
		apiMeta         apiprovider.InstanceMetadata
		clusterARN      string
		clusterARNErr   error
		expectedOutput  map[string]any
		expectedLookups []string
	}{
		{
			name: "arn and cluster name enabled",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
				K8sClusterName:   metadata.ResourceAttributeConfig{Enabled: true},
			},
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
				AccountID:  "123456789012",
			},
			apiMeta:    apiprovider.InstanceMetadata{ClusterName: "my-cluster"},
			clusterARN: "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster",
			expectedOutput: map[string]any{
				"aws.eks.cluster.arn": "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster",
				"k8s.cluster.name":    "my-cluster",
			},
			expectedLookups: []string{"my-cluster"},
		},
		{
			// The ARN comes back from EKS, so the attribute carries whatever
			// partition and account the cluster actually lives in rather than
			// one worked out from the node.
			name: "arn reported in another partition",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
			},
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-gov-west-1",
				AccountID:  "123456789012",
			},
			apiMeta:    apiprovider.InstanceMetadata{ClusterName: "my-cluster"},
			clusterARN: "arn:aws-us-gov:eks:us-gov-west-1:123456789012:cluster/my-cluster",
			expectedOutput: map[string]any{
				"aws.eks.cluster.arn": "arn:aws-us-gov:eks:us-gov-west-1:123456789012:cluster/my-cluster",
			},
			expectedLookups: []string{"my-cluster"},
		},
		{
			// A node tag can name a cluster that does not exist, and the role
			// may not be allowed to describe clusters at all. Either way the
			// attribute is left off instead of being made up.
			name: "cluster lookup fails",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
				K8sClusterName:   metadata.ResourceAttributeConfig{Enabled: true},
			},
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
				AccountID:  "123456789012",
			},
			apiMeta:       apiprovider.InstanceMetadata{ClusterName: "ghost-cluster"},
			clusterARNErr: errors.New("AccessDeniedException: not authorized to perform eks:DescribeCluster"),
			expectedOutput: map[string]any{
				"k8s.cluster.name": "ghost-cluster",
			},
			expectedLookups: []string{"ghost-cluster"},
		},
		{
			// A tag value that cannot name a cluster is rejected before it
			// costs a request.
			name: "cluster name is not usable",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
			},
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
				AccountID:  "123456789012",
			},
			apiMeta:         apiprovider.InstanceMetadata{ClusterName: "team/cluster"},
			clusterARN:      "arn:aws:eks:us-west-2:123456789012:cluster/team",
			expectedOutput:  map[string]any{},
			expectedLookups: nil,
		},
		{
			// Tags that name no cluster, or that name more than one at the
			// same precedence, leave both attributes off rather than
			// reporting a cluster named "".
			name: "no single cluster name on the node",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
				K8sClusterName:   metadata.ResourceAttributeConfig{Enabled: true},
			},
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
				AccountID:  "123456789012",
			},
			apiMeta:         apiprovider.InstanceMetadata{ClusterName: ""},
			expectedOutput:  map[string]any{},
			expectedLookups: nil,
		},
		{
			// The default config leaves the ARN disabled, so existing pipelines
			// keep the attributes they had and make no extra request.
			name: "arn disabled",
			ra: metadata.ResourceAttributesConfig{
				K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
			},
			imdsMeta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
				AccountID:  "123456789012",
			},
			apiMeta:    apiprovider.InstanceMetadata{ClusterName: "my-cluster"},
			clusterARN: "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster",
			expectedOutput: map[string]any{
				"k8s.cluster.name": "my-cluster",
			},
			expectedLookups: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiProvider := &mockAPIProvider{
				apiMeta:       tt.apiMeta,
				clusterARN:    tt.clusterARN,
				clusterARNErr: tt.clusterARNErr,
			}
			d := &detector{
				logger:       zaptest.NewLogger(t),
				imdsProvider: &mockIMDSProvider{meta: tt.imdsMeta},
				apiProvider:  apiProvider,
				rb:           metadata.NewResourceBuilder(tt.ra),
				ra:           tt.ra,
			}
			res, schema, err := d.detectFromIMDS(t.Context())
			require.NoError(t, err)
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
			assert.Equal(t, tt.expectedLookups, apiProvider.clusterARNLookups)
		})
	}
}

func TestDetectClusterARNFromIMDSFailsWhenLookupIsRequired(t *testing.T) {
	ra := metadata.ResourceAttributesConfig{
		AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
	}
	d := &detector{
		logger: zaptest.NewLogger(t),
		imdsProvider: &mockIMDSProvider{
			meta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
				AccountID:  "123456789012",
			},
		},
		apiProvider:           &mockAPIProvider{apiErr: errors.New("describe instances failed")},
		rb:                    metadata.NewResourceBuilder(ra),
		ra:                    ra,
		failOnMissingMetadata: true,
	}

	resource, schema, err := d.detectFromIMDS(t.Context())

	require.ErrorContains(t, err, "describe instances failed")
	assert.Empty(t, schema)
	assert.Equal(t, 0, resource.Attributes().Len())
}

// TestClusterARNFailureFollowsFailOnMissingMetadata pins the one rule that
// decides what an unprovable ARN does: it is fatal when
// fail_on_missing_metadata is set and omitted when it is not. The attribute
// gets no exemption from the flag, and the flag gets no exception for it.
func TestClusterARNFailureFollowsFailOnMissingMetadata(t *testing.T) {
	ra := metadata.ResourceAttributesConfig{
		AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
		K8sClusterName:   metadata.ResourceAttributeConfig{Enabled: true},
	}
	reasons := []struct {
		name            string
		clusterName     string
		clusterARNErr   error
		expectedLookups []string
		// The complete attribute set when the failure is not fatal.
		expectedOutput map[string]any
	}{
		{
			name:            "lookup rejected",
			clusterName:     "my-cluster",
			clusterARNErr:   errors.New("AccessDeniedException: not authorized to perform eks:DescribeCluster"),
			expectedLookups: []string{"my-cluster"},
			expectedOutput:  map[string]any{"k8s.cluster.name": "my-cluster"},
		},
		{
			// The check proved the name belongs to another cluster, so the
			// name is dropped along with the ARN. Keeping it would attribute
			// this node's telemetry to that cluster.
			name:            "named cluster is a different cluster",
			clusterName:     "my-cluster",
			clusterARNErr:   apiprovider.ErrClusterIdentityMismatch,
			expectedLookups: []string{"my-cluster"},
			expectedOutput:  map[string]any{},
		},
		{
			name:            "cluster name cannot name a cluster",
			clusterName:     "team/cluster",
			expectedLookups: nil,
			expectedOutput:  map[string]any{"k8s.cluster.name": "team/cluster"},
		},
		{
			name:            "no single cluster name on the node",
			clusterName:     "",
			expectedLookups: nil,
			expectedOutput:  map[string]any{},
		},
	}
	paths := []struct {
		name   string
		detect func(*detector, context.Context) (pcommon.Resource, string, error)
	}{
		{
			name: "imds",
			detect: func(d *detector, ctx context.Context) (pcommon.Resource, string, error) {
				return d.detectFromIMDS(ctx)
			},
		},
		{
			name: "kubernetes api",
			detect: func(d *detector, ctx context.Context) (pcommon.Resource, string, error) {
				return d.detectFromAPI(ctx)
			},
		},
	}
	for _, path := range paths {
		for _, reason := range reasons {
			for _, failOnMissing := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/fail_on_missing_metadata=%t", path.name, reason.name, failOnMissing)
				t.Run(name, func(t *testing.T) {
					apiProvider := &mockAPIProvider{
						nodeName: "node-1",
						k8sMeta: apiprovider.InstanceMetadata{
							Region:     "us-west-2",
							InstanceID: "i-123",
						},
						apiMeta: apiprovider.InstanceMetadata{
							Region:      "us-west-2",
							InstanceID:  "i-123",
							ClusterName: reason.clusterName,
						},
						clusterARNErr: reason.clusterARNErr,
					}
					d := &detector{
						logger: zaptest.NewLogger(t),
						imdsProvider: &mockIMDSProvider{
							meta: imds.InstanceIdentityDocument{
								InstanceID: "i-123",
								Region:     "us-west-2",
								AccountID:  "123456789012",
							},
						},
						apiProvider:           apiProvider,
						rb:                    metadata.NewResourceBuilder(ra),
						ra:                    ra,
						failOnMissingMetadata: failOnMissing,
					}

					resource, schema, err := path.detect(d, t.Context())

					if failOnMissing {
						require.Error(t, err)
						assert.ErrorContains(t, err, "eks cluster ARN unavailable")
						// The name can come from a tag anyone can write, so it
						// must not be repeated in the error the collector
						// reports.
						if reason.clusterName != "" {
							assert.NotContains(t, err.Error(), reason.clusterName)
						}
						assert.Empty(t, schema)
						assert.Equal(t, 0, resource.Attributes().Len())
						return
					}
					require.NoError(t, err)
					assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
					assert.Equal(t, reason.expectedOutput, resource.Attributes().AsRaw())
					assert.Equal(t, reason.expectedLookups, apiProvider.clusterARNLookups)
				})
			}
		}
	}
}

// TestClusterARNDisabledIsNeverFatal checks that leaving the attribute off, as
// the default config does, cannot fail detection however fail_on_missing_metadata
// is set: nothing was asked for, so nothing is missing.
func TestClusterARNDisabledIsNeverFatal(t *testing.T) {
	ra := metadata.ResourceAttributesConfig{
		K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
	}
	for _, failOnMissing := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail_on_missing_metadata=%t", failOnMissing), func(t *testing.T) {
			apiProvider := &mockAPIProvider{
				apiMeta:       apiprovider.InstanceMetadata{ClusterName: "my-cluster"},
				clusterARNErr: errors.New("eks:DescribeCluster would have failed"),
			}
			d := &detector{
				logger: zaptest.NewLogger(t),
				imdsProvider: &mockIMDSProvider{
					meta: imds.InstanceIdentityDocument{
						InstanceID: "i-123",
						Region:     "us-west-2",
						AccountID:  "123456789012",
					},
				},
				apiProvider:           apiProvider,
				rb:                    metadata.NewResourceBuilder(ra),
				ra:                    ra,
				failOnMissingMetadata: failOnMissing,
			}

			resource, _, err := d.detectFromIMDS(t.Context())

			require.NoError(t, err)
			assert.Equal(t, map[string]any{"k8s.cluster.name": "my-cluster"}, resource.Attributes().AsRaw())
			assert.Nil(t, apiProvider.clusterARNLookups)
		})
	}
}

// TestClusterNameOmittedWithARNDisabled covers the configuration that the
// omission actually changes for existing users: the ARN left disabled, as the
// default config does, and a node whose tags establish no single cluster name.
// The attribute is absent rather than present and empty, and no lookup is made.
func TestClusterNameOmittedWithARNDisabled(t *testing.T) {
	ra := metadata.ResourceAttributesConfig{
		K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
	}
	for _, failOnMissing := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail_on_missing_metadata=%t", failOnMissing), func(t *testing.T) {
			apiProvider := &mockAPIProvider{
				apiMeta: apiprovider.InstanceMetadata{
					Region:               "us-west-2",
					InstanceID:           "i-123",
					ClusterName:          "",
					ClusterNameAmbiguous: true,
				},
			}
			d := &detector{
				logger: zaptest.NewLogger(t),
				imdsProvider: &mockIMDSProvider{
					meta: imds.InstanceIdentityDocument{
						InstanceID: "i-123",
						Region:     "us-west-2",
						AccountID:  "123456789012",
					},
				},
				apiProvider:           apiProvider,
				rb:                    metadata.NewResourceBuilder(ra),
				ra:                    ra,
				failOnMissingMetadata: failOnMissing,
			}

			resource, _, err := d.detectFromIMDS(t.Context())

			require.NoError(t, err)
			assert.Equal(t, map[string]any{}, resource.Attributes().AsRaw())
			assert.Nil(t, apiProvider.clusterARNLookups)
		})
	}
}

// TestDetectDoesNotInheritAttributesFromAFailedRun covers the builder being
// reused across runs. Detect is retried on failure and re-run on every
// refresh_interval tick, so a run that failed after setting attributes must not
// leave them behind for the next one.
func TestDetectDoesNotInheritAttributesFromAFailedRun(t *testing.T) {
	t.Setenv(kubernetesServiceHostEnvVar, "10.0.0.1")
	t.Setenv(awsWebIdentityTokenFileEnvVar, "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
	ra := metadata.ResourceAttributesConfig{
		AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
		K8sClusterName:   metadata.ResourceAttributeConfig{Enabled: true},
	}
	apiProvider := &mockAPIProvider{
		apiMeta: apiprovider.InstanceMetadata{
			Region:      "us-west-2",
			InstanceID:  "i-123",
			ClusterName: "first-cluster",
		},
		clusterARNErr: errors.New("AccessDeniedException: not authorized to perform eks:DescribeCluster"),
	}
	d := &detector{
		logger: zaptest.NewLogger(t),
		imdsProvider: &mockIMDSProvider{
			meta: imds.InstanceIdentityDocument{
				InstanceID: "i-123",
				Region:     "us-west-2",
				AccountID:  "123456789012",
			},
		},
		apiProvider:           apiProvider,
		rb:                    metadata.NewResourceBuilder(ra),
		ra:                    ra,
		utils:                 &mockDetectorUtils{imdsAccessible: true},
		failOnMissingMetadata: true,
	}

	// First run sets the cluster name, then fails on the ARN before emitting.
	_, _, err := d.Detect(t.Context())
	require.Error(t, err)

	// Second run establishes no cluster name at all. It must not report the
	// name the failed run had already put in the builder.
	apiProvider.apiMeta.ClusterName = ""
	apiProvider.clusterARNErr = nil
	d.failOnMissingMetadata = false

	resource, _, err := d.Detect(t.Context())

	require.NoError(t, err)
	attrs := resource.Attributes().AsRaw()
	assert.NotContains(t, attrs, "k8s.cluster.name")
	assert.NotContains(t, attrs, "aws.eks.cluster.arn")
}

func TestDetectClusterARNFromAPI(t *testing.T) {
	tests := []struct {
		name            string
		ra              metadata.ResourceAttributesConfig
		k8sMeta         apiprovider.InstanceMetadata
		apiMeta         apiprovider.InstanceMetadata
		clusterARN      string
		clusterARNErr   error
		expectedOutput  map[string]any
		expectedLookups []string
	}{
		{
			name: "arn enabled",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
				K8sClusterName:   metadata.ResourceAttributeConfig{Enabled: true},
			},
			k8sMeta: apiprovider.InstanceMetadata{Region: "us-west-2", InstanceID: "i-123"},
			apiMeta: apiprovider.InstanceMetadata{
				Region:      "us-west-2",
				InstanceID:  "i-123",
				AccountID:   "123456789012",
				ClusterName: "my-cluster",
			},
			clusterARN: "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster",
			expectedOutput: map[string]any{
				"aws.eks.cluster.arn": "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster",
				"k8s.cluster.name":    "my-cluster",
			},
			expectedLookups: []string{"my-cluster"},
		},
		{
			// With no cluster name there is nothing to describe, so no request
			// is made.
			name: "missing cluster name",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
			},
			k8sMeta: apiprovider.InstanceMetadata{Region: "us-west-2", InstanceID: "i-123"},
			apiMeta: apiprovider.InstanceMetadata{
				Region:     "us-west-2",
				InstanceID: "i-123",
				AccountID:  "123456789012",
			},
			expectedOutput:  map[string]any{},
			expectedLookups: nil,
		},
		{
			name: "cluster lookup fails",
			ra: metadata.ResourceAttributesConfig{
				AwsEksClusterArn: metadata.ResourceAttributeConfig{Enabled: true},
				K8sClusterName:   metadata.ResourceAttributeConfig{Enabled: true},
			},
			k8sMeta: apiprovider.InstanceMetadata{Region: "us-west-2", InstanceID: "i-123"},
			apiMeta: apiprovider.InstanceMetadata{
				Region:      "us-west-2",
				InstanceID:  "i-123",
				AccountID:   "123456789012",
				ClusterName: "ghost-cluster",
			},
			clusterARNErr: errors.New("ResourceNotFoundException: No cluster found for name: ghost-cluster"),
			expectedOutput: map[string]any{
				"k8s.cluster.name": "ghost-cluster",
			},
			expectedLookups: []string{"ghost-cluster"},
		},
		{
			name: "arn disabled",
			ra: metadata.ResourceAttributesConfig{
				K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
			},
			k8sMeta: apiprovider.InstanceMetadata{Region: "us-west-2", InstanceID: "i-123"},
			apiMeta: apiprovider.InstanceMetadata{
				Region:      "us-west-2",
				InstanceID:  "i-123",
				AccountID:   "123456789012",
				ClusterName: "my-cluster",
			},
			clusterARN: "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster",
			expectedOutput: map[string]any{
				"k8s.cluster.name": "my-cluster",
			},
			expectedLookups: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiProvider := &mockAPIProvider{
				k8sMeta:       tt.k8sMeta,
				apiMeta:       tt.apiMeta,
				nodeName:      "node",
				clusterARN:    tt.clusterARN,
				clusterARNErr: tt.clusterARNErr,
			}
			d := &detector{
				logger:      zaptest.NewLogger(t),
				apiProvider: apiProvider,
				rb:          metadata.NewResourceBuilder(tt.ra),
				ra:          tt.ra,
			}
			res, schema, err := d.detectFromAPI(t.Context())
			require.NoError(t, err)
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
			assert.Equal(t, tt.expectedLookups, apiProvider.clusterARNLookups)
		})
	}
}

type mockDetectorUtils struct {
	cfg            Config
	logger         *zap.Logger
	imdsAccessible bool
}

func (m *mockDetectorUtils) isIMDSAccessible(_ context.Context) bool {
	return m.imdsAccessible
}

func (*mockDetectorUtils) getAWSConfig(context.Context, bool) (aws.Config, error) {
	return aws.Config{}, nil
}

func TestDetect(t *testing.T) {
	// Minimal config for detector
	cfg := Config{
		ResourceAttributes: metadata.ResourceAttributesConfig{
			K8sClusterName: metadata.ResourceAttributeConfig{Enabled: true},
			HostName:       metadata.ResourceAttributeConfig{Enabled: true},
			HostID:         metadata.ResourceAttributeConfig{Enabled: true},
		},
	}
	t.Setenv(kubernetesServiceHostEnvVar, "1")

	set := processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	}

	det := &detector{
		cfg:    cfg,
		logger: set.Logger,
		apiProvider: &mockAPIProvider{
			clusterVersion: "v1.32.3-eks-d0fe756",
			nodeName:       "i-123",
			apiMeta: apiprovider.InstanceMetadata{
				ClusterName:      "cluster",
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
				Hostname:         "i-123",
			},
			k8sMeta: apiprovider.InstanceMetadata{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
			},
		},
		imdsProvider: &mockIMDSProvider{
			meta: imds.InstanceIdentityDocument{
				InstanceID:       "i-123",
				AvailabilityZone: "us-west-2a",
				Region:           "us-west-2",
				AccountID:        "acc",
				ImageID:          "ami-123",
				InstanceType:     "t3.medium",
			},
			hostname: "i-123",
		},
		ra: cfg.ResourceAttributes,
		rb: metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}

	tests := []struct {
		name           string
		imdsAccessible bool
		expectedOutput map[string]any
	}{
		{
			name:           "Detect from API",
			imdsAccessible: false,
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "i-123",
			},
		},
		{
			name:           "Detect from IMDS",
			imdsAccessible: true,
			expectedOutput: map[string]any{
				"k8s.cluster.name": "cluster",
				"host.id":          "i-123",
				"host.name":        "i-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			det.utils = &mockDetectorUtils{
				cfg:            cfg,
				logger:         set.Logger,
				imdsAccessible: tt.imdsAccessible,
			}
			res, schema, err := det.Detect(t.Context())
			assert.NoError(t, err)
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			assert.Equal(t, tt.expectedOutput, res.Attributes().AsRaw())
		})
	}
}

func TestDetectFailOnMissingMetadata(t *testing.T) {
	cfg := Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
	isEKSErr := errors.New("cluster version unavailable")

	set := processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	}

	det := &detector{
		cfg:    cfg,
		logger: set.Logger,
		// apiProvider returns an error from ClusterVersion, causing isEKS() to fail.
		// KUBERNETES_SERVICE_HOST must be set so isEKS() reaches ClusterVersion().
		apiProvider: &mockAPIProvider{
			clusterVersionErr: isEKSErr,
			oidcIssuerErr:     errors.New("no oidc"),
		},
		imdsProvider: &mockIMDSProvider{},
		ra:           cfg.ResourceAttributes,
		rb:           metadata.NewResourceBuilder(cfg.ResourceAttributes),
		utils: &mockDetectorUtils{
			cfg:    cfg,
			logger: set.Logger,
		},
	}

	t.Setenv(kubernetesServiceHostEnvVar, "1")

	t.Run("err != nil and FailOnMissingMetadata=false suppresses error", func(t *testing.T) {
		res, schema, err := det.Detect(t.Context())
		assert.NoError(t, err)
		assert.Empty(t, schema)
		assert.Equal(t, 0, res.Attributes().Len())
	})

	t.Run("err != nil and FailOnMissingMetadata=true returns error", func(t *testing.T) {
		det.failOnMissingMetadata = true
		_, _, err := det.Detect(t.Context())
		assert.ErrorContains(t, err, "eks metadata unavailable")
		assert.ErrorContains(t, err, isEKSErr.Error())
	})
}

func TestDetectNotEKSFailOnMissingMetadata(t *testing.T) {
	t.Setenv(kubernetesServiceHostEnvVar, "")
	cfg := CreateDefaultConfig()
	det := &detector{
		cfg:                   cfg,
		logger:                zap.NewNop(),
		apiProvider:           &mockAPIProvider{},
		imdsProvider:          &mockIMDSProvider{},
		ra:                    cfg.ResourceAttributes,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		utils:                 &mockDetectorUtils{},
		failOnMissingMetadata: true,
	}

	res, schemaURL, err := det.Detect(t.Context())
	require.EqualError(t, err, "eks metadata unavailable: not running on EKS")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestNewDetectorOutsideCluster(t *testing.T) {
	isolateAWSConfig(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.ErrorContains(t, err, "Could not load cluster config")
	assert.Nil(t, d)
}

func TestIsIMDSAccessible(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected bool
	}{
		{name: "accessible", status: http.StatusOK, expected: true},
		{name: "not accessible", status: http.StatusNotFound, expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAWSConfig(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Connection", "close")
				if r.Method == http.MethodPut && r.URL.Path == "/latest/api/token" {
					w.Header().Set("X-Aws-Ec2-Metadata-Token-Ttl-Seconds", "21600")
					_, _ = w.Write([]byte("token"))
					return
				}
				if r.URL.Path == "/latest/meta-data/instance-id" && tt.status == http.StatusOK {
					_, _ = w.Write([]byte("i-123"))
					return
				}
				w.WriteHeader(tt.status)
			}))
			t.Cleanup(srv.Close)
			t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", srv.URL)

			utils := eksDetectorUtils{cfg: CreateDefaultConfig(), logger: zap.NewNop()}
			assert.Equal(t, tt.expected, utils.isIMDSAccessible(t.Context()))
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	assert.Equal(t, Config{ResourceAttributes: metadata.DefaultResourceAttributesConfig()}, CreateDefaultConfig())
}

// isolateAWSConfig pins every part of the environment the AWS SDK reads, so
// these tests describe only what the test itself sets up. Without this a host
// that disables IMDS, points it somewhere else, or exports a container or
// web-identity credential source decides the outcome, and the test passes or
// fails for reasons that have nothing to do with the code under test.
func isolateAWSConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", dir+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", dir+"/credentials")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_REGION", "us-east-1")

	// IMDS reachability must come from the test's own endpoint, never from the
	// host's opinion about whether IMDS exists or where it lives.
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_V1_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT_MODE", "IPv4")
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", "")

	// A credential source exported by the host would otherwise be resolved
	// ahead of the test's, and can fail config loading outright.
	t.Setenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "")
	t.Setenv("AWS_CONTAINER_CREDENTIALS_FULL_URI", "")
	t.Setenv("AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
	t.Setenv("AWS_ROLE_ARN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_SESSION_TOKEN", "")

	// A CA bundle or an endpoint override exported by the host can fail config
	// loading outright or send the request somewhere else.
	t.Setenv("AWS_CA_BUNDLE", "")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_EC2", "")
	t.Setenv("AWS_ENDPOINT_URL_EKS", "")
	t.Setenv("AWS_USE_FIPS_ENDPOINT", "false")
	t.Setenv("AWS_USE_DUALSTACK_ENDPOINT", "false")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "")
}
