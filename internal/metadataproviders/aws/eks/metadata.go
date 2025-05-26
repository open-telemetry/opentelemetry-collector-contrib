// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/eks"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	clusterNameAwsEksTag     = "aws:eks:cluster-name"
	clusterNameEksTag        = "eks:cluster-name"
	kubernetesClusterNameTag = "kubernetes.io/cluster/"
)

type Provider interface {
	ClusterVersion() (string, error)
	GetK8sInstanceMetadata(ctx context.Context) (InstanceMetadata, error)
	GetInstanceMetadata(ctx context.Context) (InstanceMetadata, error)
	SetRegionInstanceID(region, instanceID string)
}

type ec2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type metadataClient struct {
	instanceMetadata InstanceMetadata
	clientset        kubernetes.Interface
	ec2Client        ec2Client
	nodeName         string
}

type InstanceMetadata struct {
	AvailabilityZone string
	AccountID        string
	InstanceID       string
	Region           string
	ClusterName      string
	ImageID          string
	InstanceType     string
	Hostname         string
}

var _ Provider = (*metadataClient)(nil)

func NewProvider(cfg aws.Config, nodeName string) (Provider, error) {
	conf, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider. Could not load cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider. Could not create clientset for Kubernetes client: %w", err)
	}

	return &metadataClient{
		instanceMetadata: InstanceMetadata{},
		nodeName:         nodeName,
		clientset:        clientset,
		ec2Client:        ec2.NewFromConfig(cfg),
	}, nil
}

func (c *metadataClient) ClusterVersion() (string, error) {
	serverVersion, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve cluster version: %w", err)
	}
	return serverVersion.GitVersion, nil
}

// GetK8sInstanceMetadata retrieves region, instanceID, and availabilityZone attributes from the K8s node's providerID.
// It requires the node name which is passed to the constructor.
// If region or instanceID are not found, it returns an error as they are mandatory to query ec2 API for the full metadata.
func (c *metadataClient) GetK8sInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	if c.nodeName == "" {
		return InstanceMetadata{}, errors.New("can't get K8s Instance Metadata; node name is empty")
	}

	node, err := c.clientset.CoreV1().Nodes().Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("can't get K8s Instance Metadata; failed to retrieve k8s node %w", err)
	}

	region, availabilityZone, instanceID := parseRegionAndInstanceID(node.Spec.ProviderID)
	if region == "" || instanceID == "" {
		return InstanceMetadata{}, fmt.Errorf("failed to retrieve region or instanceId from mockProviderID: %s", node.Spec.ProviderID)
	}

	c.instanceMetadata = InstanceMetadata{Region: region, InstanceID: instanceID, AvailabilityZone: availabilityZone}
	return c.instanceMetadata, nil
}

func parseRegionAndInstanceID(input string) (string, string, string) {
	// Example mockProviderID: "aws:///us-west-2a/i-049ca2df511bec762"
	parts := strings.Split(input, "/")
	// Prevent NPE and return empty strings if the format is invalid
	if len(parts) < 5 || parts[0] != "aws:" || len(parts[3]) == 0 || len(parts[4]) == 0 {
		return "", "", ""
	}

	availabilityZone := parts[3]
	region := availabilityZone[:len(availabilityZone)-1] // Remove the last character (zone letter)
	instanceID := parts[4]

	return region, availabilityZone, instanceID
}

// GetInstanceMetadata retrieves detailed instance metadata from AWS EC2.
// It requires region and InstanceID to be set, if not set, it calls GetK8sInstanceMetadata to set them
func (c *metadataClient) GetInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	if c.instanceMetadata.Region == "" || c.instanceMetadata.InstanceID == "" {
		if _, err := c.GetK8sInstanceMetadata(ctx); err != nil {
			return InstanceMetadata{}, err
		}
	}
	instances, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{
			c.instanceMetadata.InstanceID,
		},
	}, func(o *ec2.Options) {
		o.Region = c.instanceMetadata.Region
	})
	if err != nil {
		return c.instanceMetadata, fmt.Errorf("failed to get instance metadata: %w", err)
	}

	if len(instances.Reservations) > 0 && len(instances.Reservations[0].Instances) > 0 {
		c.instanceMetadata.AccountID = aws.ToString(instances.Reservations[0].OwnerId)
		c.instanceMetadata.ImageID = aws.ToString(instances.Reservations[0].Instances[0].ImageId)
		c.instanceMetadata.InstanceType = string(instances.Reservations[0].Instances[0].InstanceType)
		c.instanceMetadata.Hostname = aws.ToString(instances.Reservations[0].Instances[0].PrivateDnsName)
	}
	c.instanceMetadata.ClusterName = getClusterNameTagFromReservations(instances.Reservations)

	return c.instanceMetadata, nil
}

func getClusterNameTagFromReservations(reservations []types.Reservation) string {
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				key := aws.ToString(tag.Key)
				if key == "" {
					continue
				}
				if key == clusterNameAwsEksTag || key == clusterNameEksTag {
					return aws.ToString(tag.Value)
				} else if strings.HasPrefix(key, kubernetesClusterNameTag) {
					return strings.TrimPrefix(key, kubernetesClusterNameTag)
				}
			}
		}
	}
	return ""
}

// SetRegionInstanceID sets the region and instance ID.
// This method is useful when caller has the region and instanceID and wants to fetch the ec2 metadata
// without calling the kube api through GetK8sInstanceMetadata first.
func (c *metadataClient) SetRegionInstanceID(region, instanceID string) {
	c.instanceMetadata.Region = region
	c.instanceMetadata.InstanceID = instanceID
}
