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
	"github.com/aws/aws-sdk-go-v2/service/sts"
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
	AccountID(ctx context.Context) (string, error)
	ClusterName(ctx context.Context, instanceID string) (string, error)
	ClusterVersion() (string, error)
	InstanceID() string
	AvailabilityZone() string
	Region() string
	InitializeInstanceMetadata(ctx context.Context, nodeName string) error
}

type metadataClient struct {
	instanceMetadata instanceMetadata
	clientset        *kubernetes.Clientset
	ec2Client        *ec2.Client
	stsClient        *sts.Client
}

type instanceMetadata struct {
	availabilityZone string
	instanceID       string
	region           string
}

var _ Provider = (*metadataClient)(nil)

func NewProvider(cfg aws.Config) (Provider, error) {
	conf, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider. Could not load cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider. Could not create clientset for Kubernetes client: %w", err)
	}

	return &metadataClient{
		clientset: clientset,
		ec2Client: ec2.NewFromConfig(cfg),
		stsClient: sts.NewFromConfig(cfg),
	}, nil
}

// InitializeInstanceMetadata should be called right after NewProvider to:
// 1. Get the instanceID which is needed for the ec2 describe call
// 2. Get the realm which is needed to pin the ec2 and sts call to a specific region.
func (c *metadataClient) InitializeInstanceMetadata(ctx context.Context, nodeName string) error {
	if nodeName == "" {
		return errors.New("node name is not set")
	}

	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve node %w", err)
	}

	region, availabilityZone, instanceID := parseRegionAndInstanceID(node.Spec.ProviderID)
	// attributes needed for the remaining calls to work
	if region == "" || instanceID == "" {
		return fmt.Errorf("failed to retrieve region or instanceId from providerID: %s", node.Spec.ProviderID)
	}

	c.instanceMetadata = instanceMetadata{availabilityZone: availabilityZone, instanceID: instanceID, region: region}
	return nil
}

func parseRegionAndInstanceID(input string) (string, string, string) {
	// Example providerID: "aws:///us-west-2a/i-049ca2df511bec762"
	parts := strings.Split(input, "/")
	// Prevent NPE and return empty strings if the format is invalid
	if len(parts) < 5 || len(parts[3]) == 0 || len(parts[4]) == 0 {
		return "", "", ""
	}

	availabilityZone := parts[3]
	region := availabilityZone[:len(availabilityZone)-1] // Remove the last character (zone letter)
	instanceID := parts[4]

	return region, availabilityZone, instanceID
}

func (c *metadataClient) AccountID(ctx context.Context) (string, error) {
	identity, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.Options) {
		o.Region = c.instanceMetadata.region
	})
	if err != nil {
		return "", fmt.Errorf("failed to get account ID: %w", err)
	}
	return aws.ToString(identity.Account), nil
}

func (c *metadataClient) ClusterName(ctx context.Context, instanceID string) (string, error) {
	instances, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{
			instanceID,
		},
	}, func(o *ec2.Options) {
		o.Region = c.instanceMetadata.region
	})
	if err != nil {
		return "", fmt.Errorf("failed to get EKS cluster name: %w", err)
	}

	clusterName := getClusterNameTagFromReservations(instances.Reservations)
	return clusterName, nil
}

func getClusterNameTagFromReservations(reservations []types.Reservation) string {
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag.Key == nil {
					continue
				}

				if *tag.Key == clusterNameAwsEksTag || *tag.Key == clusterNameEksTag {
					return *tag.Value
				} else if strings.HasPrefix(*tag.Key, kubernetesClusterNameTag) {
					return strings.TrimPrefix(*tag.Key, kubernetesClusterNameTag)
				}
			}
		}
	}
	return ""
}

func (c *metadataClient) ClusterVersion() (string, error) {
	serverVersion, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve cluster version: %w", err)
	}
	return serverVersion.GitVersion, nil
}

func (c *metadataClient) InstanceID() string {
	return c.instanceMetadata.instanceID
}

func (c *metadataClient) AvailabilityZone() string {
	return c.instanceMetadata.availabilityZone
}

func (c *metadataClient) Region() string {
	return c.instanceMetadata.region
}
