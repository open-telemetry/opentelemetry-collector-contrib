// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	testClusterName = "example-cluster"
	testClusterARN  = "arn:aws:eks:us-west-2:123456789012:cluster/example-cluster"
	testIssuer      = "https://oidc.eks.us-west-2.amazonaws.com/id/EXAMPLE"
)

type mockEKSClient struct {
	output *eks.DescribeClusterOutput
	err    error
	name   string
	region string
}

func (m *mockEKSClient) DescribeCluster(_ context.Context, input *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	m.name = aws.ToString(input.Name)
	options := &eks.Options{}
	for _, optFn := range optFns {
		optFn(options)
	}
	m.region = options.Region
	return m.output, m.err
}

func oidcClient(t *testing.T, status int, issuer string) kubernetes.Interface {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/openid-configuration", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, err := w.Write([]byte(`{"issuer":"` + issuer + `"}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	client, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	require.NoError(t, err)
	return client
}

func describedCluster(arn, issuer string) *eks.DescribeClusterOutput {
	return &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{
		Arn: aws.String(arn),
		Identity: &ekstypes.Identity{Oidc: &ekstypes.OIDC{
			Issuer: aws.String(issuer),
		}},
	}}
}

func TestClusterARN(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		region      string
		status      int
		liveIssuer  string
		output      *eks.DescribeClusterOutput
		apiErr      error
		wantARN     string
		wantError   string
	}{
		{
			name:        "matching issuer",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusOK,
			liveIssuer:  testIssuer,
			output:      describedCluster(testClusterARN, testIssuer+"/"),
			wantARN:     testClusterARN,
		},
		{
			name:        "issuer mismatch",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusOK,
			liveIssuer:  testIssuer,
			output:      describedCluster(testClusterARN, "https://example.invalid/other"),
			wantError:   "OIDC issuer does not match",
		},
		{
			name:        "describe failure",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusOK,
			liveIssuer:  testIssuer,
			apiErr:      errors.New("access denied"),
			wantError:   "failed to describe EKS cluster",
		},
		{
			name:        "missing described issuer",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusOK,
			liveIssuer:  testIssuer,
			output:      &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{}},
			wantError:   "response has no OIDC issuer",
		},
		{
			name:        "missing ARN",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusOK,
			liveIssuer:  testIssuer,
			output:      describedCluster("", testIssuer),
			wantError:   "response has no ARN",
		},
		{
			name:        "live issuer unavailable",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusForbidden,
			wantError:   "failed to retrieve OIDC configuration",
		},
		{
			name:        "live issuer is empty",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusOK,
			wantError:   "live EKS OIDC issuer is empty",
		},
		{
			name:        "nil cluster response",
			clusterName: testClusterName,
			region:      "us-west-2",
			status:      http.StatusOK,
			liveIssuer:  testIssuer,
			wantError:   "response has no cluster",
		},
		{
			name:      "missing cluster name",
			region:    "us-west-2",
			status:    http.StatusOK,
			wantError: "cluster name is empty",
		},
		{
			name:        "missing region",
			clusterName: testClusterName,
			status:      http.StatusOK,
			wantError:   "cluster region is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockEKSClient{output: tt.output, err: tt.apiErr}
			provider := &metadataClient{
				instanceMetadata: InstanceMetadata{Region: tt.region, ClusterName: tt.clusterName},
				clientset:        oidcClient(t, tt.status, tt.liveIssuer),
				eksClient:        client,
			}

			arn, err := provider.ClusterARN(t.Context(), tt.region, tt.clusterName)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantError)
				assert.Empty(t, arn)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantARN, arn)
			assert.Equal(t, tt.clusterName, client.name)
			assert.Equal(t, tt.region, client.region)
		})
	}
}
