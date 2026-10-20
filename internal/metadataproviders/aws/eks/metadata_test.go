// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// testCredentials stands in for a credentials provider so that the real EKS
// client can sign a request against a local endpoint. It is written out here
// rather than taken from the SDK's credentials module, which this package does
// not otherwise need and would only pull in for tests.
type testCredentials struct{}

func (testCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "secret",
		Source:          "testCredentials",
	}, nil
}

var _ aws.CredentialsProvider = testCredentials{}

func TestGetK8sInstanceMetadata(t *testing.T) {
	clientset := fake.NewClientset()
	mockEC2 := ec2.NewFromConfig(aws.Config{})

	tests := []struct {
		testName     string
		testNodeName string
		nodeName     string
		providerID   string
		region       string
		az           string
		instanceID   string
		errMsg       string
	}{
		{
			testName:     "valid node name",
			testNodeName: "test-node",
			nodeName:     "test-node",
			providerID:   "aws:///us-west-2a/i-1234567890abcdef0",
			region:       "us-west-2",
			az:           "us-west-2a",
			instanceID:   "i-1234567890abcdef0",
			errMsg:       "",
		},
		{
			testName:     "invalid - node name not set",
			testNodeName: "test-nod-not-set",
			nodeName:     "",
			errMsg:       "can't get K8s Instance Metadata; node name is empty",
		},
		{
			testName:     "invalid node name",
			testNodeName: "test-node-invalid",
			nodeName:     "non-existent-node",
			errMsg:       "can't get K8s Instance Metadata; failed to retrieve k8s node nodes \"non-existent-node\" not found",
		},
		{
			testName:     "invalid provider ID",
			testNodeName: "test-node-invalid-provider-id",
			nodeName:     "test-node-invalid-provider-id",
			providerID:   "aws://s/s",
			errMsg:       "failed to retrieve region or instanceId from mockProviderID: aws://s/s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			provider := &metadataClient{
				instanceMetadata: InstanceMetadata{},
				clientset:        clientset,
				ec2Client:        mockEC2,
				nodeName:         tt.nodeName,
			}
			err := setupNodeProviderID(clientset, tt.providerID, tt.testNodeName)
			assert.NoError(t, err)
			k8sInstanceMetadata, err := provider.GetK8sInstanceMetadata(t.Context())
			if tt.errMsg != "" {
				assert.EqualError(t, err, tt.errMsg)
			} else {
				assert.Equal(t, tt.region, k8sInstanceMetadata.Region)
				assert.Equal(t, tt.az, k8sInstanceMetadata.AvailabilityZone)
				assert.Equal(t, tt.instanceID, k8sInstanceMetadata.InstanceID)
			}
		})
	}
}

func setupNodeProviderID(client *fake.Clientset, provideID, nodeName string) error {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: corev1.NodeSpec{
			ProviderID: provideID,
		},
	}
	_, err := client.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	time.Sleep(2 * time.Millisecond)

	return nil
}

type mockEC2Client struct {
	DescribeInstancesFunc func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return m.DescribeInstancesFunc(ctx, params, optFns...)
}

func TestGetInstanceMetadata(t *testing.T) {
	type mockEC2DescOutput struct {
		instanceID   string
		clusterName  string
		ownerID      string
		imageID      string
		instanceType string
		hostname     string
	}

	mockClient := func(ec2DescOutput mockEC2DescOutput) *mockEC2Client {
		return &mockEC2Client{
			DescribeInstancesFunc: func(_ context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				if params.InstanceIds[0] != ec2DescOutput.instanceID {
					return nil, errors.New("failed to describe instances")
				}
				for _, optFn := range optFns {
					options := &ec2.Options{}
					optFn(options)
					if options.Region == "" {
						return nil, errors.New("Invalid Configuration: Missing Region")
					}
				}
				output := &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							OwnerId: aws.String(ec2DescOutput.ownerID),
							Instances: []types.Instance{
								{
									InstanceId:     aws.String(ec2DescOutput.instanceID),
									ImageId:        aws.String(ec2DescOutput.imageID),
									InstanceType:   types.InstanceType(ec2DescOutput.instanceType),
									PrivateDnsName: aws.String(ec2DescOutput.hostname),
									Tags: []types.Tag{
										{
											Key:   aws.String(clusterNameEksTag),
											Value: aws.String(ec2DescOutput.clusterName),
										},
									},
								},
							},
						},
					},
				}
				return output, nil
			},
		}
	}

	tests := []struct {
		testName       string
		ec2DescOutput  mockEC2DescOutput
		region         string
		instanceID     string
		nodeName       string
		mockNodeName   string
		mockProviderID string
		errMsg         string
	}{
		{
			testName: "valid test",
			ec2DescOutput: mockEC2DescOutput{
				instanceID:   "i-1234567890abcdef0",
				clusterName:  "test-cluster",
				ownerID:      "123456789012",
				imageID:      "ami-12345678",
				instanceType: "t2.micro",
				hostname:     "ip-192-168-1-1.us-west-2.compute.internal",
			},
			region:     "us-west-2",
			instanceID: "i-1234567890abcdef0",
			errMsg:     "",
		},
		{
			testName: "invalid instance ID",
			ec2DescOutput: mockEC2DescOutput{
				instanceID:   "i-1234567890abcdef0",
				clusterName:  "test-cluster",
				ownerID:      "123456789012",
				imageID:      "ami-12345678",
				instanceType: "t2.micro",
				hostname:     "ip-192-168-1-1.us-west-2.compute.internal",
			},
			instanceID: "i-0987654321abcdef0",
			region:     "us-west-2",
			errMsg:     "failed to describe instances",
		},
		{
			testName: "valid - missing region and instanceID",
			ec2DescOutput: mockEC2DescOutput{
				instanceID:   "i-1234567890abcdef0",
				clusterName:  "test-cluster",
				ownerID:      "123456789012",
				imageID:      "ami-12345678",
				instanceType: "t2.micro",
				hostname:     "ip-192-168-1-1.us-west-2.compute.internal",
			},
			mockProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
			nodeName:       "test-node",
			mockNodeName:   "test-node",
			errMsg:         "",
		},
		{
			testName: "invalid - missing node name",
			ec2DescOutput: mockEC2DescOutput{
				instanceID:   "i-1234567890abcdef0",
				clusterName:  "test-cluster",
				ownerID:      "123456789012",
				imageID:      "ami-12345678",
				instanceType: "t2.micro",
				hostname:     "ip-192-168-1-1.us-west-2.compute.internal",
			},
			mockProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
			mockNodeName:   "test-node",
			errMsg:         "can't get K8s Instance Metadata; node name is empty",
		},
		{
			testName: "invalid - empty region/instanceID",
			ec2DescOutput: mockEC2DescOutput{
				instanceID:   "i-1234567890abcdef0",
				clusterName:  "test-cluster",
				ownerID:      "123456789012",
				imageID:      "ami-12345678",
				instanceType: "t2.micro",
				hostname:     "ip-192-168-1-1.us-west-2.compute.internal",
			},
			mockProviderID: "aws:///",
			mockNodeName:   "test-node",
			nodeName:       "test-node",
			errMsg:         "failed to retrieve region or instanceId from mockProviderID: aws:///",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			clientset := fake.NewClientset()
			eksProvider := &metadataClient{
				ec2Client: mockClient(tt.ec2DescOutput),
				clientset: clientset,
				instanceMetadata: InstanceMetadata{
					Region:     tt.region,
					InstanceID: tt.instanceID,
				},
				nodeName: tt.nodeName,
			}
			err := setupNodeProviderID(clientset, tt.mockProviderID, tt.mockNodeName)
			assert.NoError(t, err)
			ec2InstanceMetadata, err := eksProvider.GetInstanceMetadata(t.Context())
			if tt.errMsg != "" && err != nil {
				assert.ErrorContains(t, err, tt.errMsg)
			} else {
				assert.Equal(t, tt.ec2DescOutput.clusterName, ec2InstanceMetadata.ClusterName)
				assert.Equal(t, tt.ec2DescOutput.ownerID, ec2InstanceMetadata.AccountID)
				assert.Equal(t, tt.ec2DescOutput.imageID, ec2InstanceMetadata.ImageID)
				assert.Equal(t, tt.ec2DescOutput.instanceType, ec2InstanceMetadata.InstanceType)
				assert.Equal(t, tt.ec2DescOutput.hostname, ec2InstanceMetadata.Hostname)
			}
		})
	}
}

func reservationWithTags(tags map[string]string) []types.Reservation {
	instanceTags := make([]types.Tag, 0, len(tags))
	for key, value := range tags {
		instanceTags = append(instanceTags, types.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	return reservationWithOrderedTags(instanceTags)
}

func reservationWithOrderedTags(instanceTags []types.Tag) []types.Reservation {
	return []types.Reservation{
		{
			OwnerId:   aws.String("123456789012"),
			Instances: []types.Instance{{Tags: instanceTags}},
		},
	}
}

func tag(key, value string) types.Tag {
	return types.Tag{Key: aws.String(key), Value: aws.String(value)}
}

// permutationsOf returns every ordering of the given tags, so a precedence
// claim can be checked against all of them instead of against whichever order
// a map range happened to produce.
func permutationsOf(tags []types.Tag) [][]types.Tag {
	if len(tags) <= 1 {
		return [][]types.Tag{append([]types.Tag(nil), tags...)}
	}
	var out [][]types.Tag
	for i := range tags {
		rest := make([]types.Tag, 0, len(tags)-1)
		rest = append(rest, tags[:i]...)
		rest = append(rest, tags[i+1:]...)
		for _, permutation := range permutationsOf(rest) {
			out = append(out, append([]types.Tag{tags[i]}, permutation...))
		}
	}
	return out
}

func TestGetClusterNameTagFromReservations(t *testing.T) {
	tests := []struct {
		name      string
		tags      map[string]string
		expected  string
		ambiguous bool
	}{
		{
			name:     "no tags",
			tags:     nil,
			expected: "",
		},
		{
			name: "only unrelated tags",
			tags: map[string]string{
				"Name":                      "worker-node",
				"kubernetes.io/role/worker": "1",
				"eks:nodegroup-name":        "ng-1",
			},
			expected: "",
		},
		{
			name: "aws eks tag wins over the eks tag",
			tags: map[string]string{
				clusterNameAwsEksTag: "owned-cluster",
				clusterNameEksTag:    "nodegroup-cluster",
			},
			expected: "owned-cluster",
		},
		{
			// The EKS-owned tag lives in a namespace users cannot write to, so
			// it must win over a discovery tag anyone can add. Picking by tag
			// order instead would let the EC2 response decide.
			name: "aws eks tag wins over a kubernetes discovery tag",
			tags: map[string]string{
				kubernetesClusterNameTag + "attacker-cluster": "owned",
				clusterNameAwsEksTag:                          "real-cluster",
			},
			expected: "real-cluster",
		},
		{
			name: "eks tag wins over a kubernetes discovery tag",
			tags: map[string]string{
				kubernetesClusterNameTag + "other-cluster": "owned",
				clusterNameEksTag:                          "real-cluster",
			},
			expected: "real-cluster",
		},
		{
			name: "owned discovery tag wins over a shared one",
			tags: map[string]string{
				kubernetesClusterNameTag + "aaa-shared-cluster": "shared",
				kubernetesClusterNameTag + "zzz-owned-cluster":  "owned",
			},
			expected: "zzz-owned-cluster",
		},
		{
			name: "discovery tag used when no eks tag is present",
			tags: map[string]string{
				kubernetesClusterNameTag + "only-cluster": "owned",
			},
			expected: "only-cluster",
		},
		{
			name: "empty tag values are skipped",
			tags: map[string]string{
				clusterNameAwsEksTag: "",
				clusterNameEksTag:    "fallback-cluster",
			},
			expected: "fallback-cluster",
		},
		{
			name: "bare discovery prefix is not a cluster name",
			tags: map[string]string{
				kubernetesClusterNameTag: "owned",
			},
			expected: "",
		},
		{
			// Two tags of the same strength naming different clusters do not
			// say which cluster the node is in. Answering anyway, even
			// consistently, would let whoever wrote the second tag decide.
			name: "conflicting owned discovery tags are ambiguous",
			tags: map[string]string{
				kubernetesClusterNameTag + "cluster-b": "owned",
				kubernetesClusterNameTag + "cluster-a": "owned",
				kubernetesClusterNameTag + "cluster-c": "owned",
			},
			ambiguous: true,
		},
		{
			name: "conflicting shared discovery tags are ambiguous",
			tags: map[string]string{
				kubernetesClusterNameTag + "cluster-a": "shared",
				kubernetesClusterNameTag + "cluster-b": "shared",
			},
			ambiguous: true,
		},
		{
			// A discovery tag is still used when it is the only claim on the
			// node, so the precedence does not cost names it should report.
			name: "a lone discovery tag is still used",
			tags: map[string]string{
				kubernetesClusterNameTag + "discovery-cluster": "owned",
			},
			expected: "discovery-cluster",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The reservations are rebuilt on every iteration so that Go's
			// randomized map range produces a different tag order each time.
			// Building them once would only re-check one fixed order.
			for range 50 {
				name, err := getClusterNameTagFromReservations(reservationWithTags(tt.tags))
				if tt.ambiguous {
					require.ErrorIs(t, err, errAmbiguousClusterName)
					assert.Empty(t, name)
					continue
				}
				require.NoError(t, err)
				assert.Equal(t, tt.expected, name)
			}
		})
	}
}

// TestGetClusterNameTagFromReservationsOverEveryTagOrder checks the precedence
// and the ambiguity rule against every ordering of the tags rather than
// relying on a random map range to eventually produce the ordering that would
// break them.
func TestGetClusterNameTagFromReservationsOverEveryTagOrder(t *testing.T) {
	tests := []struct {
		name      string
		tags      []types.Tag
		expected  string
		ambiguous bool
	}{
		{
			name: "every tier present at once",
			tags: []types.Tag{
				tag(clusterNameAwsEksTag, "eks-owned-cluster"),
				tag(clusterNameEksTag, "nodegroup-cluster"),
				tag(kubernetesClusterNameTag+"owned-cluster", "owned"),
				tag(kubernetesClusterNameTag+"shared-cluster", "shared"),
			},
			expected: "eks-owned-cluster",
		},
		{
			name: "nodegroup tag beats both discovery tags",
			tags: []types.Tag{
				tag(clusterNameEksTag, "nodegroup-cluster"),
				tag(kubernetesClusterNameTag+"owned-cluster", "owned"),
				tag(kubernetesClusterNameTag+"shared-cluster", "shared"),
			},
			expected: "nodegroup-cluster",
		},
		{
			name: "owned discovery tag beats shared ones",
			tags: []types.Tag{
				tag(kubernetesClusterNameTag+"shared-a", "shared"),
				tag(kubernetesClusterNameTag+"owned-cluster", "owned"),
				tag(kubernetesClusterNameTag+"shared-b", "shared"),
			},
			expected: "owned-cluster",
		},
		{
			name: "aws eks tag beats the nodegroup tag whatever the order",
			tags: []types.Tag{
				tag(clusterNameAwsEksTag, "cluster-a"),
				tag(clusterNameEksTag, "nodegroup-cluster"),
			},
			expected: "cluster-a",
		},
		{
			name: "two owned discovery tags are ambiguous whatever the order",
			tags: []types.Tag{
				tag(kubernetesClusterNameTag+"cluster-a", "owned"),
				tag(kubernetesClusterNameTag+"cluster-b", "owned"),
			},
			ambiguous: true,
		},
		{
			// Ambiguity in the strongest tier present must not be resolved by
			// dropping to the next tier, which anyone can write.
			name: "ambiguous owned tags are not resolved by a shared tag",
			tags: []types.Tag{
				tag(kubernetesClusterNameTag+"cluster-a", "owned"),
				tag(kubernetesClusterNameTag+"cluster-b", "owned"),
				tag(kubernetesClusterNameTag+"cluster-c", "shared"),
			},
			ambiguous: true,
		},
		{
			// EC2 tag keys are unique per instance, so the same tag is only
			// ever seen twice across instances. That shape is covered by
			// TestGetClusterNameTagFromReservationsAcrossInstances.
			name: "a single owned tag is used",
			tags: []types.Tag{
				tag(kubernetesClusterNameTag+"cluster-a", "owned"),
			},
			expected: "cluster-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := permutationsOf(tt.tags)
			require.NotEmpty(t, orders)
			for _, order := range orders {
				name, err := getClusterNameTagFromReservations(reservationWithOrderedTags(order))
				if tt.ambiguous {
					require.ErrorIs(t, err, errAmbiguousClusterName, "order %v", order)
					assert.Empty(t, name)
					continue
				}
				require.NoError(t, err, "order %v", order)
				assert.Equal(t, tt.expected, name, "order %v", order)
			}
		})
	}
}

// TestGetClusterNameTagFromReservationsAcrossInstances covers the shape the
// response actually has: every instance in it carries the node's tags, so the
// same tag key is seen once per instance. Repeats of the same name are not a
// conflict, and this is the only shape in which the two EKS tiers, whose keys
// are unique per instance, can conflict at all.
func TestGetClusterNameTagFromReservationsAcrossInstances(t *testing.T) {
	reservationsWith := func(perInstance ...[]types.Tag) []types.Reservation {
		instances := make([]types.Instance, 0, len(perInstance))
		for _, tags := range perInstance {
			instances = append(instances, types.Instance{Tags: tags})
		}
		return []types.Reservation{{OwnerId: aws.String("123456789012"), Instances: instances}}
	}

	t.Run("the same eks tag on two instances is not ambiguous", func(t *testing.T) {
		name, err := getClusterNameTagFromReservations(reservationsWith(
			[]types.Tag{tag(clusterNameAwsEksTag, "real-cluster")},
			[]types.Tag{tag(clusterNameAwsEksTag, "real-cluster")},
		))
		require.NoError(t, err)
		assert.Equal(t, "real-cluster", name)
	})

	t.Run("different eks tags across instances are ambiguous", func(t *testing.T) {
		name, err := getClusterNameTagFromReservations(reservationsWith(
			[]types.Tag{tag(clusterNameAwsEksTag, "cluster-a")},
			[]types.Tag{tag(clusterNameAwsEksTag, "cluster-b")},
		))
		require.ErrorIs(t, err, errAmbiguousClusterName)
		assert.Empty(t, name)
	})

	t.Run("different nodegroup tags across instances are ambiguous", func(t *testing.T) {
		name, err := getClusterNameTagFromReservations(reservationsWith(
			[]types.Tag{tag(clusterNameEksTag, "cluster-a")},
			[]types.Tag{tag(clusterNameEksTag, "cluster-b")},
		))
		require.ErrorIs(t, err, errAmbiguousClusterName)
		assert.Empty(t, name)
	})

	t.Run("spread across separate reservations", func(t *testing.T) {
		name, err := getClusterNameTagFromReservations([]types.Reservation{
			{Instances: []types.Instance{{Tags: []types.Tag{tag(clusterNameEksTag, "cluster-a")}}}},
			{Instances: []types.Instance{{Tags: []types.Tag{tag(clusterNameEksTag, "cluster-b")}}}},
		})
		require.ErrorIs(t, err, errAmbiguousClusterName)
		assert.Empty(t, name)
	})
}

type mockEKSClient struct {
	DescribeClusterFunc func(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

func (m *mockEKSClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return m.DescribeClusterFunc(ctx, params, optFns...)
}

// oidcClientset returns a Kubernetes client whose OIDC discovery document is
// served locally, so the provider reads the live cluster's issuer over the same
// code path it uses in a cluster rather than from a stub.
func oidcClientset(t *testing.T, issuer string, status int) kubernetes.Interface {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = fmt.Fprintf(w, `{"issuer":%q}`, issuer)
		}
	}))
	t.Cleanup(srv.Close)
	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	require.NoError(t, err)
	return clientset
}

const (
	testIssuer      = "https://oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B716D3041E"
	testOtherIssuer = "https://oidc.eks.us-west-2.amazonaws.com/id/0000000000000000000000000000000A"
)

func clusterWithIssuer(arn, issuer string) *ekstypes.Cluster {
	return &ekstypes.Cluster{
		Name:     aws.String("my-cluster"),
		Arn:      aws.String(arn),
		Identity: &ekstypes.Identity{Oidc: &ekstypes.OIDC{Issuer: aws.String(issuer)}},
	}
}

func TestClusterARN(t *testing.T) {
	const arn = "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster"
	tests := []struct {
		name             string
		region           string
		clusterName      string
		liveIssuer       string
		oidcStatus       int
		output           *eks.DescribeClusterOutput
		outputErr        error
		expected         string
		wantErr          error
		wantDescribeCall bool
	}{
		{
			name:             "cluster is the one this node runs in",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			output:           &eks.DescribeClusterOutput{Cluster: clusterWithIssuer(arn, testIssuer)},
			expected:         arn,
			wantDescribeCall: true,
		},
		{
			// The ARN is whatever EKS reports, so a GovCloud or China cluster
			// does not depend on the collector knowing the partition.
			name:             "cluster found in another partition",
			region:           "cn-north-1",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			output:           &eks.DescribeClusterOutput{Cluster: clusterWithIssuer("arn:aws-cn:eks:cn-north-1:123456789012:cluster/my-cluster", testIssuer)},
			expected:         "arn:aws-cn:eks:cn-north-1:123456789012:cluster/my-cluster",
			wantDescribeCall: true,
		},
		{
			// A trailing slash does not name a different cluster, so it must
			// not cost the attribute.
			name:             "issuers differ only by a trailing slash",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer + "/",
			output:           &eks.DescribeClusterOutput{Cluster: clusterWithIssuer(arn, testIssuer)},
			expected:         arn,
			wantDescribeCall: true,
		},
		{
			// This is the case a name on its own cannot catch: the cluster
			// exists, in this account and Region, and is not this node's.
			name:             "named cluster exists but is a different cluster",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			output:           &eks.DescribeClusterOutput{Cluster: clusterWithIssuer("arn:aws:eks:us-west-2:123456789012:cluster/someone-elses", testOtherIssuer)},
			wantErr:          ErrClusterIdentityMismatch,
			wantDescribeCall: true,
		},
		{
			name:             "described cluster carries no identity",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			output:           &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{Arn: aws.String(arn)}},
			wantErr:          errDescribedIssuerUnavailable,
			wantDescribeCall: true,
		},
		{
			name:        "described cluster carries no oidc issuer",
			region:      "us-west-2",
			clusterName: "my-cluster",
			liveIssuer:  testIssuer,
			output: &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{
				Arn:      aws.String(arn),
				Identity: &ekstypes.Identity{},
			}},
			wantErr:          errDescribedIssuerUnavailable,
			wantDescribeCall: true,
		},
		{
			name:             "described oidc issuer is empty",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			output:           &eks.DescribeClusterOutput{Cluster: clusterWithIssuer(arn, "")},
			wantErr:          errDescribedIssuerUnavailable,
			wantDescribeCall: true,
		},
		{
			// With no issuer to check the response against there is nothing to
			// prove, so the request is not worth making.
			name:        "live issuer unavailable",
			region:      "us-west-2",
			clusterName: "my-cluster",
			oidcStatus:  http.StatusInternalServerError,
			wantErr:     errLiveIssuerUnavailable,
		},
		{
			name:        "live issuer is empty",
			region:      "us-west-2",
			clusterName: "my-cluster",
			liveIssuer:  "",
			wantErr:     errLiveIssuerUnavailable,
		},
		{
			name:             "cluster does not exist",
			region:           "us-west-2",
			clusterName:      "ghost-cluster",
			liveIssuer:       testIssuer,
			outputErr:        &ekstypes.ResourceNotFoundException{Message: aws.String("No cluster found for name: ghost-cluster.")},
			wantErr:          errDescribeClusterFailed,
			wantDescribeCall: true,
		},
		{
			name:             "not allowed to describe the cluster",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			outputErr:        errors.New("AccessDeniedException: User is not authorized to perform: eks:DescribeCluster"),
			wantErr:          errDescribeClusterFailed,
			wantDescribeCall: true,
		},
		{
			name:             "response carries no cluster",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			output:           &eks.DescribeClusterOutput{},
			wantErr:          errNoClusterInResponse,
			wantDescribeCall: true,
		},
		{
			name:             "response carries an empty arn",
			region:           "us-west-2",
			clusterName:      "my-cluster",
			liveIssuer:       testIssuer,
			output:           &eks.DescribeClusterOutput{Cluster: clusterWithIssuer("", testIssuer)},
			wantErr:          errNoClusterARN,
			wantDescribeCall: true,
		},
		{
			name:        "no cluster name",
			region:      "us-west-2",
			clusterName: "",
			liveIssuer:  testIssuer,
			wantErr:     errNoClusterName,
		},
		{
			name:        "no region",
			clusterName: "my-cluster",
			liveIssuer:  testIssuer,
			wantErr:     errNoRegion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.oidcStatus
			if status == 0 {
				status = http.StatusOK
			}
			called := false
			provider := &metadataClient{
				instanceMetadata: InstanceMetadata{Region: tt.region},
				clientset:        oidcClientset(t, tt.liveIssuer, status),
				eksClient: &mockEKSClient{
					DescribeClusterFunc: func(_ context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
						called = true
						assert.Equal(t, tt.clusterName, aws.ToString(params.Name))
						// The cluster lives in the node's Region, so the
						// request must be sent there and not to whatever
						// Region the default config resolved.
						options := &eks.Options{}
						for _, optFn := range optFns {
							optFn(options)
						}
						assert.Equal(t, tt.region, options.Region)
						return tt.output, tt.outputErr
					},
				},
			}

			arn, err := provider.ClusterARN(t.Context(), tt.clusterName)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, arn)
				// The name can come from a tag anyone can write, so it must
				// not be repeated in anything the collector reports. Nor must
				// the issuer URLs, which name the clusters involved.
				if tt.clusterName != "" {
					assert.NotContains(t, err.Error(), tt.clusterName)
				}
				if tt.liveIssuer != "" {
					assert.NotContains(t, err.Error(), tt.liveIssuer)
				}
				assert.NotContains(t, err.Error(), testOtherIssuer)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, arn)
			}
			assert.Equal(t, tt.wantDescribeCall, called, "DescribeCluster call")
		})
	}
}

// TestGetInstanceMetadataPrefersTheEKSOwnedTag covers the whole provider path,
// so a node carrying both an EKS-owned tag and a user-written discovery tag
// reports the cluster EKS says it belongs to.
func TestGetInstanceMetadataPrefersTheEKSOwnedTag(t *testing.T) {
	clientset := fake.NewClientset()
	provider := &metadataClient{
		instanceMetadata: InstanceMetadata{Region: "us-west-2", InstanceID: "i-1234567890abcdef0"},
		clientset:        clientset,
		ec2Client: &mockEC2Client{
			DescribeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{
					Reservations: reservationWithTags(map[string]string{
						kubernetesClusterNameTag + "unrelated-cluster": "owned",
						clusterNameEksTag:    "nodegroup-cluster",
						clusterNameAwsEksTag: "real-cluster",
					}),
				}, nil
			},
		},
	}

	meta, err := provider.GetInstanceMetadata(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "real-cluster", meta.ClusterName)
	assert.Equal(t, "123456789012", meta.AccountID)
}

// TestGetInstanceMetadataWithAmbiguousClusterTags covers the whole provider
// path for a node whose tags name more than one cluster at the same
// precedence: the cluster name is dropped, and the rest of the node's metadata
// is still reported.
func TestGetInstanceMetadataWithAmbiguousClusterTags(t *testing.T) {
	provider := &metadataClient{
		instanceMetadata: InstanceMetadata{Region: "us-west-2", InstanceID: "i-1234567890abcdef0"},
		clientset:        fake.NewClientset(),
		ec2Client: &mockEC2Client{
			DescribeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{
					Reservations: reservationWithTags(map[string]string{
						kubernetesClusterNameTag + "cluster-a": "owned",
						kubernetesClusterNameTag + "cluster-b": "owned",
					}),
				}, nil
			},
		},
	}

	meta, err := provider.GetInstanceMetadata(t.Context())

	require.NoError(t, err)
	assert.Empty(t, meta.ClusterName)
	assert.Equal(t, "123456789012", meta.AccountID)
}

// TestGetInstanceMetadataDoesNotReuseAnEarlierResponse covers the provider
// being reused across detection runs. Detect is retried on failure and re-run
// on every refresh_interval tick against the same provider, so a later
// ec2:DescribeInstances response that succeeds while describing no instance
// must not be reported with the values the previous response carried.
func TestGetInstanceMetadataDoesNotReuseAnEarlierResponse(t *testing.T) {
	describedInstance := &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				OwnerId: aws.String("123456789012"),
				Instances: []types.Instance{
					{
						InstanceId:     aws.String("i-1234567890abcdef0"),
						ImageId:        aws.String("ami-12345678"),
						InstanceType:   types.InstanceType("t2.micro"),
						PrivateDnsName: aws.String("ip-192-168-1-1.us-west-2.compute.internal"),
						Tags: []types.Tag{
							{Key: aws.String(clusterNameAwsEksTag), Value: aws.String("my-cluster")},
						},
					},
				},
			},
		},
	}

	// Every response an EC2 call can succeed with while describing no
	// instance. Each one has to clear what the first response established
	// rather than leaving it to be reported again.
	emptyResponses := []struct {
		name   string
		output *ec2.DescribeInstancesOutput
	}{
		{name: "no reservations", output: &ec2.DescribeInstancesOutput{}},
		{
			name:   "reservation with no instances",
			output: &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{{OwnerId: aws.String("123456789012")}}},
		},
	}

	for _, empty := range emptyResponses {
		t.Run(empty.name, func(t *testing.T) {
			response := describedInstance
			provider := &metadataClient{
				instanceMetadata: InstanceMetadata{Region: "us-west-2", InstanceID: "i-1234567890abcdef0"},
				clientset:        fake.NewClientset(),
				ec2Client: &mockEC2Client{
					DescribeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
						return response, nil
					},
				},
			}

			first, err := provider.GetInstanceMetadata(t.Context())
			require.NoError(t, err)
			require.Equal(t, "123456789012", first.AccountID)
			require.Equal(t, "ami-12345678", first.ImageID)
			require.Equal(t, "t2.micro", first.InstanceType)
			require.Equal(t, "ip-192-168-1-1.us-west-2.compute.internal", first.Hostname)
			require.Equal(t, "my-cluster", first.ClusterName)

			response = empty.output
			second, err := provider.GetInstanceMetadata(t.Context())

			require.NoError(t, err)
			assert.Empty(t, second.AccountID)
			assert.Empty(t, second.ImageID)
			assert.Empty(t, second.InstanceType)
			assert.Empty(t, second.Hostname)
			assert.Empty(t, second.ClusterName)
			// The region and instance ID are the inputs to the call rather
			// than values read out of the response, so they are kept.
			assert.Equal(t, "us-west-2", second.Region)
			assert.Equal(t, "i-1234567890abcdef0", second.InstanceID)
		})
	}
}

// TestClusterARNFromRealSDKClient drives a real EKS client against a local
// endpoint, so the ARN and the issuer it is checked against are read out of an
// actual DescribeCluster response rather than values handed back by a stub.
func TestClusterARNFromRealSDKClient(t *testing.T) {
	const arn = "arn:aws:eks:us-west-2:123456789012:cluster/my-cluster"
	tests := []struct {
		name       string
		liveIssuer string
		status     int
		body       string
		expected   string
		wantErr    error
	}{
		{
			name:       "cluster is the one this node runs in",
			liveIssuer: testIssuer,
			status:     http.StatusOK,
			body:       `{"cluster":{"name":"my-cluster","arn":"` + arn + `","status":"ACTIVE","identity":{"oidc":{"issuer":"` + testIssuer + `"}}}}`,
			expected:   arn,
		},
		{
			name:       "named cluster exists but is a different cluster",
			liveIssuer: testIssuer,
			status:     http.StatusOK,
			body:       `{"cluster":{"name":"my-cluster","arn":"` + arn + `","status":"ACTIVE","identity":{"oidc":{"issuer":"` + testOtherIssuer + `"}}}}`,
			wantErr:    ErrClusterIdentityMismatch,
		},
		{
			name:       "described cluster carries no identity",
			liveIssuer: testIssuer,
			status:     http.StatusOK,
			body:       `{"cluster":{"name":"my-cluster","arn":"` + arn + `","status":"ACTIVE"}}`,
			wantErr:    errDescribedIssuerUnavailable,
		},
		{
			name:       "cluster does not exist",
			liveIssuer: testIssuer,
			status:     http.StatusNotFound,
			body:       `{"__type":"ResourceNotFoundException","message":"No cluster found for name: my-cluster."}`,
			wantErr:    errDescribeClusterFailed,
		},
		{
			name:       "not allowed to describe the cluster",
			liveIssuer: testIssuer,
			status:     http.StatusForbidden,
			body:       `{"__type":"AccessDeniedException","message":"User is not authorized to perform: eks:DescribeCluster"}`,
			wantErr:    errDescribeClusterFailed,
		},
		{
			name:       "response carries no cluster",
			liveIssuer: testIssuer,
			status:     http.StatusOK,
			body:       `{}`,
			wantErr:    errNoClusterInResponse,
		},
		{
			name:       "response is not valid json",
			liveIssuer: testIssuer,
			status:     http.StatusOK,
			body:       `{"cluster":`,
			wantErr:    errDescribeClusterFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/clusters/my-cluster")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			provider := &metadataClient{
				instanceMetadata: InstanceMetadata{Region: "us-west-2"},
				clientset:        oidcClientset(t, tt.liveIssuer, http.StatusOK),
				eksClient: eks.New(eks.Options{
					Region:       "us-west-2",
					BaseEndpoint: aws.String(srv.URL),
					Credentials:  testCredentials{},
					Retryer:      aws.NopRetryer{},
				}),
			}

			arn, err := provider.ClusterARN(t.Context(), "my-cluster")

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, arn)
				assert.NotContains(t, err.Error(), "my-cluster")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, arn)
			}
		})
	}
}

// TestOIDCIssuer covers the live issuer read on its own, since the ARN binding
// is only as good as this request.
func TestOIDCIssuer(t *testing.T) {
	t.Run("issuer reported", func(t *testing.T) {
		provider := &metadataClient{clientset: oidcClientset(t, testIssuer, http.StatusOK)}
		issuer, err := provider.OIDCIssuer(t.Context())
		require.NoError(t, err)
		assert.Equal(t, testIssuer, issuer)
	})
	t.Run("endpoint unavailable", func(t *testing.T) {
		provider := &metadataClient{clientset: oidcClientset(t, "", http.StatusInternalServerError)}
		_, err := provider.OIDCIssuer(t.Context())
		require.Error(t, err)
	})
}

func TestCanonicalIssuer(t *testing.T) {
	tests := []struct {
		name     string
		issuer   string
		expected string
	}{
		{name: "unchanged", issuer: testIssuer, expected: testIssuer},
		{name: "trailing slash removed", issuer: testIssuer + "/", expected: testIssuer},
		{name: "repeated trailing slashes removed", issuer: testIssuer + "///", expected: testIssuer},
		{name: "surrounding space removed", issuer: "  " + testIssuer + "\n", expected: testIssuer},
		{name: "empty stays empty", issuer: "", expected: ""},
		{name: "only slashes collapse to empty", issuer: "//", expected: ""},
		{
			// A different host is a different cluster, so it must survive
			// canonicalisation and fail the comparison.
			name:     "host is not rewritten",
			issuer:   "https://OIDC.eks.us-west-2.amazonaws.com/id/ABC",
			expected: "https://OIDC.eks.us-west-2.amazonaws.com/id/ABC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, canonicalIssuer(tt.issuer))
		})
	}
}
