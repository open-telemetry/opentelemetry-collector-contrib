// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetK8sInstanceMetadata(t *testing.T) {
	clientset := fake.NewSimpleClientset()
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
			k8sInstanceMetadata, err := provider.GetK8sInstanceMetadata(context.Background())
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

func setupNodeProviderID(client *fake.Clientset, provideID string, nodeName string) error {
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
			clientset := fake.NewSimpleClientset()
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
			ec2InstanceMetadata, err := eksProvider.GetInstanceMetadata(context.Background())
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
